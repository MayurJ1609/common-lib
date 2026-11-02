package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

type Options struct {
	File      string
	EnvPrefix string
}

func Load[T any](options Options) (T, error) {
	var value T
	v := viper.New()
	v.SetConfigFile(options.File)
	v.SetConfigType(configType(options.File))
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	if options.EnvPrefix != "" {
		v.SetEnvPrefix(options.EnvPrefix)
	}
	if err := bindStructEnv(v, reflect.TypeOf(value), ""); err != nil {
		return value, err
	}

	if options.File != "" {
		if err := v.ReadInConfig(); err != nil {
			return value, fmt.Errorf("read config file %q: %w", options.File, err)
		}
	}
	if err := v.Unmarshal(&value); err != nil {
		return value, fmt.Errorf("decode configuration: %w", err)
	}
	return value, nil
}

func bindStructEnv(v *viper.Viper, typ reflect.Type, parent string) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		name := strings.Split(field.Tag.Get("mapstructure"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		key := name
		if parent != "" {
			key = parent + "." + name
		}
		if field.Type.Kind() == reflect.Struct || field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct {
			if err := bindStructEnv(v, field.Type, key); err != nil {
				return err
			}
			continue
		}
		if err := v.BindEnv(key); err != nil {
			return fmt.Errorf("bind environment variable for %q: %w", key, err)
		}
	}
	return nil
}

func configType(path string) string {
	if path == "" {
		return "yaml"
	}
	if index := strings.LastIndexByte(path, '.'); index >= 0 {
		return path[index+1:]
	}
	return "yaml"
}
