package snmptrap

import (
    "fmt"
    "strconv"
    "sync/atomic"

    "github.com/k-sone/snmpgo"
    "github.com/pkg/errors"
)

type Config struct {
    // The host:port address of the SNMP trap server
    Addr string 
    // SNMP Community
    Community string 
    // Retries count for traps
    Retries uint 
}

type Service struct {
    configValue atomic.Value
    client      *snmpgo.SNMP
}

//type Options struct {
//    Options         HandlerConfig    `yaml:"options,omitempty" json:"options,omitempty"`
//}

type Options  struct {
    TrapOid         string             `yaml:"trap-oid,omitempty" json:"trap-oid,omitempty"`
    DataList        []Data             `yaml:"data-list,omitempty" json:"data-list,omitempty"`
}

type Data struct {
    Oid             string             `yaml:"oid,omitempty" json:"oid,omitempty"`
    Type            string             `yaml:"type,omitempty" json:"type,omitempty"`
    Value           string             `yaml:"value,omitempty" json:"value,omitempty"`
}

func NewService(c Config) *Service {
    s := &Service{}
    s.configValue.Store(c)
    return s
}

func (s *Service) Open() error {
    c := s.config()
    err := s.loadNewSNMPClient(c)
    if err != nil {
        return err
    }
    return nil
}

func (s *Service) Close() error {
    s.closeClient()
    return nil
}

func (s *Service) closeClient() {
    if s.client != nil {
        s.client.Close()
    }
    s.client = nil
}

func (s *Service) config() Config {
    return s.configValue.Load().(Config)
}

func (s *Service) loadNewSNMPClient(c Config) error {
    snmp, err := snmpgo.NewSNMP(snmpgo.SNMPArguments{
        Version:   snmpgo.V2c,
        Address:   c.Addr,
        Retries:   uint(c.Retries),
        Community: c.Community,
    })
    if err != nil {
        return errors.Wrap(err, "invalid SNMP configuration")
    }
    s.client = snmp
    return nil
}

func (s *Service) Trap(trapOid string, dataList []Data) error {

    // Add trap oid
    oid, err := snmpgo.NewOid(trapOid)
    if err != nil {
        return errors.Wrapf(err, "invalid trap Oid %q", trapOid)
    }
    varBinds := snmpgo.VarBinds{
        snmpgo.NewVarBind(snmpgo.OidSysUpTime, snmpgo.NewTimeTicks(1000)),
        snmpgo.NewVarBind(snmpgo.OidSnmpTrap, oid),
    }

    // Add Data
    for _, data := range dataList {
        oid, err := snmpgo.NewOid(data.Oid)
        if err != nil {
            return errors.Wrapf(err, "invalid data Oid %q", data.Oid)
        }
        // http://docstore.mik.ua/orelly/networking_2ndEd/snmp/ch10_03.htm
        switch data.Type {
            case "a":
                return errors.New("Snmptrap Datatype 'IP address' not supported")
            case "c":
                oidValue, err := strconv.ParseInt(data.Value, 10, 64)
                if err != nil {
                    return err
                }
                varBinds = append(varBinds, snmpgo.NewVarBind(oid, snmpgo.NewCounter64(uint64(oidValue))))
            case "d":
                return errors.New("Snmptrap Datatype 'Decimal string' not supported")
            case "i":
                oidValue, err := strconv.ParseInt(data.Value, 10, 64)
                if err != nil {
                    return err
                }
                varBinds = append(varBinds, snmpgo.NewVarBind(oid, snmpgo.NewInteger(int32(oidValue))))
            case "n":
                varBinds = append(varBinds, snmpgo.NewVarBind(oid, snmpgo.NewNull()))
            case "o":
                return errors.New("Snmptrap Datatype 'Object ID' not supported")
            case "s":
                oidValue := []byte(data.Value)
                varBinds = append(varBinds, snmpgo.NewVarBind(oid, snmpgo.NewOctetString(oidValue)))
            case "t":
                oidValue, err := strconv.ParseInt(data.Value, 10, 64)
                if err != nil {
                    return err
                }
                varBinds = append(varBinds, snmpgo.NewVarBind(oid, snmpgo.NewTimeTicks(uint32(oidValue))))
            case "u":
                return errors.New("Snmptrap Datatype 'Unsigned integer' not supported")
            case "x":
                return errors.New("Snmptrap Datatype 'Hexadecimal string' not supported")
            default:
                return fmt.Errorf("Snmptrap Datatype not known: %v", data.Type)
        }
    }

    if err = s.client.V2Trap(varBinds); err != nil {
        return errors.Wrap(err, "failed to send SNMP trap")
    }
    return nil
}

