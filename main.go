package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// 全局变量
var (
	BLOG_PATH        string // 整理好的博客文件目录  EasyBlog/asset/blogs
	PICTURE_PATH     string // 整理好的图片目录  EasyBlog/asset/pictures
	YAML_FILE_PATH   string // 记录整理好的博客信息 EasyBlog/resource/blog-list.yaml
	REPOSITORY_URL   string
	LOCAL_FILE_PATH  string
	UPDATE_FILE_HASH string
)

// 可定义的配置文件，在init中赋值给全局变量
type configData struct {
	RepositoryUrl  string `json:"repository_url"`   // 作为图床的仓库地址
	LocalFilePath  string `json:"local_file_path"`  // 待整理的本地博客目录地址
	UpdateFileHash string `json:"update_file_hash"` // 待更新的博客的hash字符串
}

var ConfigData configData

func init() {
	path, _ := filepath.Abs(".")
	BLOG_PATH = filepath.Join(path, "asset", "blogs")
	PICTURE_PATH = filepath.Join(path, "asset", "pictures")
	YAML_FILE_PATH = filepath.Join(path, "resource", "blog-list.yaml")

	file, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&ConfigData)
	if err != nil {
		log.Fatal(err)
	}

	REPOSITORY_URL = ConfigData.RepositoryUrl
	LOCAL_FILE_PATH = ConfigData.LocalFilePath
	UPDATE_FILE_HASH = ConfigData.UpdateFileHash
}

func main() {
	if len(os.Args) == 1 {
		UploadBlogs()
		return
	}

	if len(os.Args) > 2 {
		fmt.Println("Too many arguments")
		return
	}

	switch os.Args[1] {
	case "upload": // 整理并提交新博客
		UploadBlogs()
	case "check", "c": // 检查博客和图片的完整性
		CheckBlogs()
	case "update", "u": // 更新已整理的单个博客
		UpdateBlog()
	}

}
