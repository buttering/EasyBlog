package main

import (
	"EasyBlogs/tools"
	"fmt"
	"os"
	"path/filepath"
)

// UpdateBlog 更新已整理的单个博客
// 0. 检查文章和图片完整性(CheckBlogs)
// 1. 根据hash名找到博客（位于已整理区域）
// 2. 重新处理文档中的图片url
// 3. 补充需要的图片
// 4. 提交更新
func UpdateBlog() {
	CheckBlogs()
	fmt.Println("")
	// 检查图片是否要归档
	var needArchive = func(picture markdownPicture) bool {
		if len(picture.sourcePicturePath) < len(REPOSITORY_URL) {
			return true
		}
		if picture.sourcePicturePath[:len(REPOSITORY_URL)] != REPOSITORY_URL { // 未经处理过
			return true
		}

		_, err := os.Stat(picture.sourcePicturePath)
		if os.IsNotExist(err) { // 图片不存在
			return true
		}
		return false
	}
	yamlContent := tools.YamlReader(YAML_FILE_PATH)
	updateBlogList := tools.Filter(yamlContent.Blogs, func(blog tools.Blog) bool {
		return blog.Hash == UPDATE_FILE_HASH
	})
	if updateBlogList == nil || len(updateBlogList) == 0 {
		fmt.Printf("没有需要更新的博客")
		return
	}

	blog := Blog{
		directoryPath: BLOG_PATH,
		name:          updateBlogList[0].Name,
		hashName:      UPDATE_FILE_HASH,
	}
	ExtractPicture(&blog)

	blog.pictures = tools.Filter(blog.pictures, func(picture markdownPicture) bool {
		return needArchive(picture)
	})
	if len(blog.pictures) == 0 {
		fmt.Println("没有需要更新的图片")
		return
	}

	blogPath := filepath.Join(blog.directoryPath, blog.name)
	content, _ := os.ReadFile(blogPath)
	offset := 0
	for _, picture := range blog.pictures {
		start := picture.start + offset
		end := picture.end + offset
		content = append(content[:start], append([]byte(picture.targetUrl), content[end:]...)...)
		offset += len(picture.targetUrl) - (end - start)
	}

	err := os.WriteFile(blogPath, content, 0644)
	if err != nil {
		println("修改文件“" + blog.name + "”失败")
		return
	}

	CopyPicture(blog)

	blogList := []Blog{blog}
	GitOperate(blogList, func(nameList []string) (summary string) {
		summary = fmt.Sprintf("修改文件 [%s]", blogList[0].name)
		return
	})
}
