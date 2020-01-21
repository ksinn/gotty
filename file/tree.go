package file

import (
	"strings"
	"errors"
	"log"
	"os"
	"path/filepath"
	"io/ioutil"
	"github.com/saintfish/chardet"

)

const LargeFileLimit int64 = 1048576 // 1 Mb

type Node struct {
	Name string			`json:"name"`
	Path string			`json:"path"`
	Type  string		`json:"type"`
	ContentType string	`json:"content_type"`
	Content string		`json:"content"`
	Children []*Node	`json:"children"`
}


func NewNode(path, name string, isDir bool, size int64) (node Node, err error) {
	node.Name = name
	node.Path = path
	if isDir == true {
		node.Type = "dir"
		node.Children = []*Node{}
	} else {
		node.Type = "file"
		var byteContent []byte
		var contentType string
		byteContent, err = ioutil.ReadFile(path)
		contentType, err = GetFileType(byteContent, size)
		node.ContentType = contentType
		if contentType == TextFileType {
			node.Content = string(byteContent)
		}
	}
	return
}


func (node *Node) addChildren(newChildren *Node) error {

	if node.Type == "file" {
		return errors.New("Node is leaf, not a branch")
	}

	node.Children = append(node.Children, newChildren)

	return nil
}


func (node *Node) getChildrenByName(name string) *Node {
	if node.Type == "file" {
		return nil
	}

	for _, children := range node.Children {

		if children.Name == name {
			return children
		}

	}

	return nil
}


func (node *Node) addToTree(newNode *Node, path string) error {
	if node.Type == "file" {
		return errors.New("Node is leaf, not a branch")
	}

	paths := strings.Split(path, "/")
	if paths[0] != node.Name {
		return errors.New("New node is not from this branch")
	}

	cutPaths := paths[1:]

	if len(cutPaths)==1 {
		node.addChildren(newNode)
	} else {
		branchName := cutPaths[0]
		branch := node.getChildrenByName(branchName)

		if branch == nil {
			newBranch, err := NewNode(node.Path+branchName, branchName, true, 0)

			if err != nil {
				return err
			}

			node.addChildren(&newBranch)

			newBranch.addChildren(newNode)

		} else {
			branch.addToTree(newNode, strings.Join(cutPaths, "/"))
		}
	}

	return nil
}


func GetFileTree() (tree *Node, err error)  {

	newTree, err := NewNode(".", ".", true, 0)
	if err != nil {
		return
	}

	tree = &newTree

	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if path != "." {

				//log.Println(info.Name())

				//if strings.HasPrefix(info.Name(), ".") {
				//	return nil
				//}

				node, err := NewNode(path, info.Name(), info.IsDir(), info.Size())

				if err != nil {
					return err
				}

				err = tree.addToTree(&node, "./"+path)

				if err != nil {
					log.Println(err)
					return err
				}
			}

			return nil
		})

	return

}

func GetFileType(content []byte, size int64) (string, error)  {
	if size > LargeFileLimit {
		return OtherFileType, nil
	}

	detector := chardet.NewTextDetector()
	result, err := detector.DetectBest(content)
	if err != nil {
		return "", err
	}

	//log.Println(result.Charset)

	//if strings.HasPrefix(result.Charset, "UTF") {
	if result.Charset == "UTF-8" || result.Charset == "ISO-8859-1" {
		return TextFileType, nil
	} else {
		return OtherFileType, nil
	}


}