package config_load

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
)

type (
	ViperLoader struct {
		*viper.Viper
		consulKey             string
		consulURL             string
		envPrefix             string
		configFileName        string
		remoteMaxAttempt      int
		tagName               string
		configFileSearchPaths []string
	}
	Option func(*ViperLoader)
)

var (
	ErrInvalidInput       = fmt.Errorf("cfg must be a pointer to struct and initialized")
	ErrConfigFileNotFound = fmt.Errorf("config file not found")
)

// WithConfigFileSearchPaths will add paths to where the loader will search the configuration files
func WithConfigFileSearchPaths(paths ...string) Option {
	return func(v *ViperLoader) {
		if len(paths) > 0 {
			v.configFileSearchPaths = append(v.configFileSearchPaths, paths...)
		}
	}
}

func WithLoadFromConsulMaxAttempt(n int) Option {
	return func(v *ViperLoader) {
		if n > 0 {
			v.remoteMaxAttempt = n
		}
	}
}

func WithConfigFileName(fileName string) Option {
	return func(v *ViperLoader) {
		v.configFileName = fileName
	}
}

func WithStructTagName(name string) Option {
	return func(v *ViperLoader) {
		v.tagName = name
	}
}

func New(envPrefix, consulKey, consulURL string, opts ...Option) *ViperLoader {
	v := &ViperLoader{
		Viper:                 viper.New(),
		configFileName:        "config",
		remoteMaxAttempt:      5,
		tagName:               "json",
		envPrefix:             envPrefix,
		consulKey:             consulKey,
		consulURL:             consulURL,
		configFileSearchPaths: []string{"."},
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

func (v *ViperLoader) Load(cfg interface{}) (err error) {
	decOption := func(dc *mapstructure.DecoderConfig) {
		dc.TagName = v.tagName
	}
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return ErrInvalidInput
	}
	if v.consulURL != "" {
		err = v.loadFromConsul()
		if err == nil {
			err = v.Unmarshal(cfg, decOption)
			return
		}
	}
	log.Printf("Can not load from consule, either consul url is not set, or an error occured: %+v. Will load configuration from file and environment variables.\n", err)
	err = v.loadFromFileAndEnv()
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		err = fmt.Errorf("%w: no '%s' file found on search paths.", ErrConfigFileNotFound, v.configFileName)
		return 
	}
	err = v.Unmarshal(cfg, decOption)
	return
}

func (v *ViperLoader) loadFromFileAndEnv() error {
	v.SetConfigName(v.configFileName)
	for _, path := range v.configFileSearchPaths {
		v.AddConfigPath(path)
	}
	v.SetEnvPrefix(v.envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	return v.ReadInConfig()
}

func (v *ViperLoader) loadFromConsul() error {
	err := v.AddRemoteProvider("consul", v.consulURL, v.consulKey)
	if err != nil {
		return err
	}

	v.SetConfigType("yaml")
	stop := false
	attempt := 0
	for !stop {
		err = v.ReadRemoteConfig()
		attempt++
		stop = err == nil || attempt >= v.remoteMaxAttempt
		if !stop {
			time.Sleep(500 * time.Millisecond)
		}

	}

	log.Printf("Initializing remote config, consul endpoint: %s, consul key: %s, number of attempt: %d", v.consulURL, v.consulKey, attempt)
	return err
}
