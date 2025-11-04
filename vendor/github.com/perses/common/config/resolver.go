// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config provides a single way to manage the configuration of your application.
// The configuration can be a yaml file and/or a list of environment variable.
// To set the config using the environment, this package is using the package github.com/nexucis/lamenv,
// which is able to determinate what is the environment variable that matched the different attribute tof the struct.
// By default it is based on the yaml tag provided.
//
// The main entry point of this package is the struct Resolver.
// This struct will allow you to set the path to your config file if you have one and to give the prefix of all of your environment variable.
// Note:
//  1. A good practice is to prefix your environment variable by the name of your application.
//  2. The config file is not mandatory, you can manage all you configuration using the environment variable.
//  3. The config by environment is always overriding the config by file.
//
// The Resolver at the end returns an object that implements the interface Validator.
// Each config/struct can implement this interface in order to provide a single way to verify the configuration and to set the default value.
// The object returned by the Resolver will loop other different structs that are parts of the config and execute the method Verify if implemented.
//
// Example:
//
//	  import (
//	          "fmt"
//
//	          "github.com/perses/common/config"
//	  )
//
//	   type Config struct {
//		    Etcd *EtcdConfig `yaml:"etcd"`
//	   }
//
//	   func (c *Config) Verify() error {
//	     if c.EtcdConfig == nil {
//	       return fmt.Errorf("etcd config cannot be empty")
//	     }
//	   }
//
//	   func Resolve(configFile string) (Config, error) {
//		    c := Config{}
//		    return c, config.NewResolver().
//			  SetConfigFile(configFile).
//			  SetEnvPrefix("PERSES").
//			  Resolve(&c).
//			  Verify()
//	   }
package config

import (
	"bytes"
	"crypto/sha1"
	"os"
	"reflect"

	"github.com/nexucis/lamenv"
	"github.com/perses/common/file"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Validator interface {
	Verify() error
}

type validatorImpl struct {
	Validator
	err    error
	config interface{}
}

// Verify will check if the different attribute of the config is implementing the interface Validator.
// If it's the case, then it will call the method Verify of each attribute.
func (v *validatorImpl) Verify() error {
	if v.err != nil {
		return v.err
	}
	ifv := reflect.ValueOf(v.config)
	return verifyRec(ifv)
}

func checkPointer(ptr reflect.Value) error {
	if ptr.IsNil() {
		return nil
	}
	if p, ok := ptr.Interface().(Validator); ok {
		if err := p.Verify(); err != nil {
			return err
		}
	}
	return nil
}

func verifyRec(conf reflect.Value) error {
	v := conf
	if conf.Kind() != reflect.Ptr {
		// that means it's not a pointer, so we have to create one to be able to then know if it implements the interface Validator
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		// so now we are able to check if the pointer is implementing the interface
		if err := checkPointer(ptr); err != nil {
			return err
		}
		// in case the method Verify() is setting some parameter in the struct, we have to save these changes
		v.Set(ptr.Elem())
	} else {
		if err := checkPointer(v); err != nil {
			return err
		}
		// for what is coming next, if it's a pointer, we need to access to the value itself
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := verifyRec(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			attr := v.Field(i)
			if len(v.Type().Field(i).PkgPath) > 0 {
				// the field is not exported, so no need to look at it as we won't be able to set it in a later stage
				continue
			}
			if err := verifyRec(attr); err != nil {
				return err
			}
		}
	}

	return nil
}

type Resolver[T any] interface {
	SetEnvPrefix(prefix string) Resolver[T]
	SetConfigFile(filename string) Resolver[T]
	SetConfigData(data []byte) Resolver[T]
	AddChangeCallback(func(*T)) Resolver[T]
	Resolve(config *T) Validator
}

type configResolver[T any] struct {
	Resolver[T]
	prefix         string
	strict         bool
	configFile     string
	data           []byte
	watchCallbacks []func(*T)
}

func NewResolver[T any]() Resolver[T] {
	return &configResolver[T]{
		strict: true,
	}
}

func (c *configResolver[T]) Strict(isStrict bool) Resolver[T] {
	c.strict = isStrict
	return c
}

func (c *configResolver[T]) SetEnvPrefix(prefix string) Resolver[T] {
	c.prefix = prefix
	return c
}

// SetConfigFile is the way to set the path to the configFile (including the name of the file)
func (c *configResolver[T]) SetConfigFile(filename string) Resolver[T] {
	c.configFile = filename
	return c
}

func (c *configResolver[T]) SetConfigData(data []byte) Resolver[T] {
	c.data = data
	return c
}

// AddChangeCallback is the way to add a callback that will be called when the config is changed
// The callback will be called with a pointer to the base config with the new values
func (c *configResolver[T]) AddChangeCallback(callback func(*T)) Resolver[T] {
	c.watchCallbacks = append(c.watchCallbacks, callback)
	return c
}

func (c *configResolver[T]) Resolve(config *T) Validator {
	err := c.read(config)
	if err == nil {
		err = lamenv.Unmarshal(config, []string{c.prefix})
		if len(c.watchCallbacks) != 0 && len(c.configFile) != 0 {
			c.watchFile(config)
		}
	}
	return &validatorImpl{
		err:    err,
		config: config,
	}
}

func (c *configResolver[T]) read(config *T) error {
	var data []byte
	var err error
	if len(c.configFile) > 0 {
		data, err = c.readFromFile()
	} else if len(c.data) > 0 {
		data = c.data
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		// config can be entirely set from environment
		return nil
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(c.strict)
	return d.Decode(config)
}

func (c *configResolver[T]) watchFile(config *T) {
	previousHash, _ := c.hashConfig(config)

	err := file.Watch(c.configFile, func() {
		var newConfig T
		err := c.read(&newConfig)
		if err != nil {
			logrus.WithError(err).Errorf("Cannot parse the watched config file %s", c.configFile)
			return
		}

		logrus.Debugln("New configuration loaded")

		newHash, _ := c.hashConfig(&newConfig)
		if previousHash == newHash {
			return
		}
		previousHash = newHash

		for _, callback := range c.watchCallbacks {
			callback(&newConfig)
		}
	})

	if err != nil {
		logrus.WithError(err).Errorf("Failed to watch the config file %s", c.configFile)
	}
}

func (c *configResolver[T]) readFromFile() ([]byte, error) {
	if len(c.configFile) == 0 {
		return nil, nil
	}
	if _, err := os.Stat(c.configFile); err == nil {
		// the file exists, so we should unmarshal the configuration using yaml
		return os.ReadFile(c.configFile)
	} else {
		return nil, err
	}
}

func (c *configResolver[T]) hashConfig(config *T) ([sha1.Size]byte, error) {
	// We don't use the file content to calculate the hash.
	//
	// The main reason is if the change doesn't affect a field
	// tracked by the config, we don't want to notify the change.
	//
	// This can happen if the struct is a part of a yaml file,
	// the change is just a syntax change, or doesn't affect the
	// value of the struct (e.g. a comment or a reordering)
	//
	// To avoid this; we have to remarshal the unmarshaled struct.
	data, err := yaml.Marshal(config)
	if err != nil {
		logrus.Errorf("Cannot marshal the config: %s", err)
		return [sha1.Size]byte{}, err
	}
	return sha1.Sum(data), err
}
