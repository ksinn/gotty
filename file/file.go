package file

import (
	"io/ioutil"
	"os"
)

func GetDirContent() (*Node, error) {
	tree, err := GetFileTree()

	if err != nil {
		return nil, err
	}

	return tree, nil

}

//TODO ограничить только текущей деректорией. запретить ходить в корень

func WriteFile(path string, content []byte) (err error) {
	err = ioutil.WriteFile(path, content, 0666)
	return
}

func RemoveFile(path string) (err error) {
	err = os.RemoveAll(path)
	return
}



