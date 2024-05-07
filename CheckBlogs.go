package main

import (
	"EasyBlogs/tools"
	"fmt"
	"log"
	"os"
	"path"
)

// CheckBlogs 检查文章和图片完整性
// 1. 生成一个新的yaml文件
// 2. 检查文章对应图片文件夹是否存在，不存在则追加
// 3. 检查是否有多余图片文件夹
func CheckBlogs() {
	blogList, err := os.ReadDir(BLOG_PATH)
	if err != nil {
		log.Fatal(err)
	}
	pictureFolderList, err := os.ReadDir(PICTURE_PATH)
	if err != nil {
		log.Fatal(err)
	}

	var yamlContent tools.BlogList
	for _, blog := range blogList {
		yamlContent.Blogs = append(yamlContent.Blogs, tools.Blog{
			Name: blog.Name(),
			Hash: tools.Hash(blog.Name()),
		})
	}
	for _, blog := range yamlContent.Blogs {
		fmt.Println(blog.Name)
		fmt.Println(blog.Hash)
	}

	tools.YamlWriter(YAML_FILE_PATH, &yamlContent)
	fmt.Println("")
	fmt.Println("重写yaml文件成功")

	hashNameListBlogs := tools.Map(yamlContent.Blogs, func(blog tools.Blog) string {
		return blog.Hash
	})
	nameListPicturesFolder := tools.Map(pictureFolderList, func(pic os.DirEntry) string {
		return pic.Name()
	})
	hashNameSetBlogs := tools.NewSet(hashNameListBlogs...)
	nameSetPicturesFolder := tools.NewSet(nameListPicturesFolder...)

	redundantPicturesFolder := nameSetPicturesFolder.Minus(hashNameSetBlogs)
	missingPicturesFolder := hashNameSetBlogs.Minus(nameSetPicturesFolder)

	for _, folderName := range redundantPicturesFolder.ToList() {
		folderPath := path.Join(PICTURE_PATH, folderName)
		err := os.RemoveAll(folderPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "err: %v\n", err)
		}
		fmt.Println("删除冗余文件夹：", folderPath)
	}
	for _, folderName := range missingPicturesFolder.ToList() {
		folderPath := path.Join(PICTURE_PATH, folderName)
		err := os.Mkdir(folderPath, 0777)
		if err != nil {
			fmt.Fprintf(os.Stderr, "err: %v\n", err)
		}
		fmt.Println("创建缺失文件夹：", folderPath)
	}
}
