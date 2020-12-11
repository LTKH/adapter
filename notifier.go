package main

import (
    "log"
    "flag"
    "bytes"
    "net/http"
    "io/ioutil"
    "runtime"
    "os"
    "os/signal"
    "syscall"
    "encoding/json"
    "text/template"
    "gopkg.in/yaml.v2"
    "gopkg.in/natefinch/lumberjack.v2"
    "github.com/ltkh/notifier/internal/snmptrap"
)

var (
    cfg *Config
)

type Config struct {
    Global           *Global            `yaml:"global" json:"global"`
    Receivers        []*Receiver        `yaml:"receivers,omitempty" json:"receivers,omitempty"`
}

type Global struct {
    ListenAddress    string             `yaml:"listen_address" json:"listen_address"`
}

// Receiver configuration provides configuration on how to contact a receiver.
type Receiver struct {
    // A unique identifier for this receiver.
    Path             string             `yaml:"path" json:"path"`

    //EmailConfigs     []*EmailConfig     `yaml:"email_configs,omitempty" json:"email_configs,omitempty"`
    //PagerdutyConfigs []*PagerdutyConfig `yaml:"pagerduty_configs,omitempty" json:"pagerduty_configs,omitempty"`
    //SlackConfigs     []*SlackConfig     `yaml:"slack_configs,omitempty" json:"slack_configs,omitempty"`
    //WebhookConfigs   []*WebhookConfig   `yaml:"webhook_configs,omitempty" json:"webhook_configs,omitempty"`
    //OpsGenieConfigs  []*OpsGenieConfig  `yaml:"opsgenie_configs,omitempty" json:"opsgenie_configs,omitempty"`
    //WechatConfigs    []*WechatConfig    `yaml:"wechat_configs,omitempty" json:"wechat_configs,omitempty"`
    //PushoverConfigs  []*PushoverConfig  `yaml:"pushover_configs,omitempty" json:"pushover_configs,omitempty"`
    //VictorOpsConfigs []*VictorOpsConfig `yaml:"victorops_configs,omitempty" json:"victorops_configs,omitempty"`
    SNMPTrapConfigs  []*SnmpTrapConfig  `yaml:"snmptrap_configs,omitempty" json:"snmptrap_configs,omitempty"`
}

// EmailConfig configures notifications via mail.
type SnmpTrapConfig struct {
    Addr             string             `yaml:"addr" json:"addr"`
    Community        string             `yaml:"community,omitempty" json:"community,omitempty"`
    Retries          uint               `yaml:"retries,omitempty" json:"retries,omitempty"`
    OptionTemplates  []string           `yaml:"option_templates,omitempty" json:"option_templates,omitempty"`
    //Options   snmptrap.HandlerConfig    `yaml:"options,omitempty" json:"options,omitempty"`
}

func webhook(w http.ResponseWriter, r *http.Request) {
  
    //reading request body
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        log.Printf("[error] %v - %s", err, r.URL.Path)
        w.WriteHeader(400)
        return
    }
    defer r.Body.Close()

    var data interface{}
    if err := json.Unmarshal(body, &data); err != nil {
        log.Printf("[error] %v - %s", err, r.URL.Path)
        w.WriteHeader(400)
        return
    }
    
    for _, receiver := range cfg.Receivers {
        if r.URL.Path == receiver.Path {
            for _, rcConf := range receiver.SNMPTrapConfigs {
                go func(rcConf *SnmpTrapConfig, data interface{}){

                    conf := snmptrap.Config{
                        Addr:      rcConf.Addr,
                        Community: rcConf.Community,
                        Retries:   1,
                    }

                    tmpl, err := template.ParseFiles(rcConf.OptionTemplates...)
                    if err != nil {
                        log.Printf("[error] %v - %s", err, rcConf.Addr)
                        return
                    }

                    var buf bytes.Buffer
                    defer buf.Reset()
                    if err = tmpl.Execute(&buf, &data); err != nil {
                        log.Printf("[error] %v - %s", err, rcConf.Addr)
                        return
                    }

                    opts := &[]snmptrap.Options{}
                    if err := yaml.UnmarshalStrict([]byte(buf.String()), opts); err != nil {
                        log.Printf("[error] parsing YAML file %v", err)
                        return
                    }

                    if len(*opts) > 0 {
                        snmp := snmptrap.NewService(conf)
                        snmp.Open()
                        for _, opt := range *opts {
                            log.Printf("[info] %v", opt.DataList)
                            snmp.Trap(opt.TrapOid, opt.DataList)
                        }
                        snmp.Close()
                    }
                    
                }(rcConf, data)
            }
        }
    }

    w.WriteHeader(204)
    return

}

func main() {

    //limits the number of operating system threads
    runtime.GOMAXPROCS(runtime.NumCPU())

    //command-line flag parsing
    cfFile          := flag.String("config", "", "config file")
    lgFile          := flag.String("logfile", "", "log file")
    log_max_size    := flag.Int("log_max_size", 1, "log max size") 
    log_max_backups := flag.Int("log_max_backups", 3, "log max backups")
    log_max_age     := flag.Int("log_max_age", 10, "log max age")
    log_compress    := flag.Bool("log_compress", true, "log compress")
    flag.Parse()

    // Logging settings
    if *lgFile != "" {
        log.SetOutput(&lumberjack.Logger{
            Filename:   *lgFile,
            MaxSize:    *log_max_size,    // megabytes after which new file is created
            MaxBackups: *log_max_backups, // number of backups
            MaxAge:     *log_max_age,     // days
            Compress:   *log_compress,    // using gzip
        })
    }

    // Loading configuration file
    content, err := ioutil.ReadFile(*cfFile)
    if err != nil {
        log.Fatalf("[error] %v", err)
    }
    
    cfg = &Config{}
    if err := yaml.UnmarshalStrict(content, cfg); err != nil {
        log.Fatalf("[error] parsing YAML file %v", err)
    }
    
    // Enabled listen port
    http.HandleFunc("/", webhook)
    go http.ListenAndServe(cfg.Global.ListenAddress, nil)

    log.Print("[info] notifier started -_-")
    
    //program completion signal processing
    c := make(chan os.Signal, 2)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)

    // Daemon mode
    for {
        <- c
        log.Print("[info] notifier stopped")
        os.Exit(0)
    }

}

