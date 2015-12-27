package config

// Local configuration from the filesystem

import (
    "path/filepath"
    "fmt"
    "io/ioutil"
    "os"
    "strings"
)

type FilesConfig struct {
    Path        string
}

type Files struct {
    config      FilesConfig
}

func (self *Files) String() string {
    return fmt.Sprintf("%s", self.config.Path)
}

func (self FilesConfig) Open() (*Files, error) {
    files := &Files{config: self}

    return files, nil
}

// Recursively any Config's under given path
func (self *Files) Scan() (configs []Config, err error) {
    err = filepath.Walk(self.config.Path, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if strings.HasPrefix(info.Name(), ".") {
            // skip
            return nil
        }

        node := Node{
            Path:   strings.Trim(strings.TrimPrefix(path, self.config.Path), "/"),
            IsDir:  info.IsDir(),
            Source: FileConfigSource,
        }

        if info.Mode().IsRegular() {
            if value, err := ioutil.ReadFile(path); err != nil {
                return err
            } else {
                node.Value = string(value)
            }
        }

        if config, err := syncConfig(node); err != nil {
            return err
        } else if config != nil {
            configs = append(configs, config)
        }

        return nil
    })

    return
}
