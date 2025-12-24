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

package file

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// Watch watches the given filename and calls the given callback when the file is changed.
// The watcher uses the parent directory as a watchpoint to be notified if the file doesn't
// exist when the watcher is created.
// Example:
//
//		file.Watch("/tmp/test.txt", func() {
//			fmt.Println("File created or changed")
//		}
//	)
func Watch(filename string, callback func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				// As we are watching the parent directory, we only care
				// about file creation and changes on the given filename.
				if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) && filepath.Base(event.Name) == filepath.Base(filename) {
					callback()
				}
			case err := <-watcher.Errors:
				if err != nil {
					logrus.WithError(err).Errorf("Unable to watch the file %s", filename)
				}
			}
		}
	}()
	// Watch the parent directory of the given filename.
	// Fix a bug with fsnotify if the file does not exist.
	err = watcher.Add(filepath.Dir(filename))
	return err
}
