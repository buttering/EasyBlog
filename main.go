package main

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type markdownPicture struct {
	isUrl       bool
	pictureName string // 源路径（包括文件名）
	start       int    // md中源图片路径的起始偏移量
	end         int
	targetName  string // 修改后的文件名（不含路径）
}

type Blog struct {
	name          string
	pictures      []markdownPicture
	directoryPath string // 源文件文件夹路径
	targetPath    string // resource中文件夹的绝对路径
}

func getBlogList(path string) (blogsList []Blog) {
	blogsList = make([]Blog, 0, 10)

	fileList, err := os.ReadDir(path)
	if err != nil {
		panic(err)
	}

	for _, file := range fileList {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".md" {
			fileName := file.Name()

			targetPath, _ := filepath.Abs(".")
			targetPath = filepath.Join(targetPath, "resource", fileName[:len(fileName)-3])
			blogsList = append(blogsList, Blog{fileName, nil, path, targetPath})
		}
	}
	return
}

func extractPicture(blog *Blog) {
	isUrl := func(path string) bool {
		return strings.HasPrefix(path, `http://`) || strings.HasPrefix(path, `https://`)
	}

	content, err := os.ReadFile(filepath.Join(blog.directoryPath, blog.name))
	if err != nil {
		println(err)
		return
	}

	re, _ := regexp.Compile(`!\[.*?]\((.*?)\)`)
	matches := re.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		start := match[2]
		end := match[3]

		picturePath := string(content[start:end])
		baseName := filepath.Base(picturePath)
		newPicturePath := baseName[:len(baseName)-len(filepath.Ext(baseName))]
		newPicturePath = fmt.Sprintf("%s%s", newPicturePath, filepath.Ext(baseName)) // 可以添加时间戳
		if !isUrl(picturePath) && !filepath.IsAbs(picturePath) {
			picturePath = filepath.Join(blog.directoryPath, picturePath)
		}

		blog.pictures = append(blog.pictures, markdownPicture{isUrl(picturePath), picturePath, start, end, newPicturePath})

	}
}

func copyBlog(blog Blog) {
	println("拷贝博客：“" + blog.name + "”")

	if _, err := os.Stat(blog.targetPath); !os.IsNotExist(err) {
		println("文章“" + blog.name + "”已经存在")
		return
	}

	if err := os.Mkdir(blog.targetPath, 0777); err != nil {
		println("创建文件夹“" + blog.name + "”失败")
		return
	}

	_ = os.Mkdir(filepath.Join(blog.targetPath, "pictures"), 0777)

	content, _ := os.ReadFile(filepath.Join(blog.directoryPath, blog.name))

	offset := 0
	for _, picture := range blog.pictures {
		start := picture.start + offset
		end := picture.end + offset
		content = append(content[:start], append([]byte(picture.targetName), content[end:]...)...)
		offset += len(picture.targetName) - len(picture.pictureName)
	}

	err := os.WriteFile(filepath.Join(blog.targetPath, blog.name), content, 0644)
	if err != nil {
		println("复制文件“" + blog.name + "”错误")
	}

}

func copyPicture(blog Blog) {

	for _, picture := range blog.pictures {
		println("导入图片：“" + picture.pictureName + "”")

		var sourceFile interface{}
		if picture.isUrl {
			for i := 0; i < 5; i++ {
				response, err := http.Get(picture.pictureName)
				if err == nil && response.StatusCode == http.StatusOK {
					sourceFile = response.Body
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			if sourceFile == nil {
				println("下载图片“" + picture.pictureName + "”失败")
				continue
			}

		} else {
			file, err := os.Open(picture.pictureName)
			if err != nil {
				println("打开图片“" + picture.pictureName + "”失败")
				continue
			}
			sourceFile = file
		}

		destinationFile, _ := os.Create(filepath.Join(blog.targetPath, "pictures", picture.targetName))

		_, err := io.Copy(destinationFile, sourceFile.(io.Reader))
		if err != nil {
			println("复制图片“" + picture.pictureName + "”失败")
		}
	}
}

func gitOperate() {
	repositoryPath, _ := filepath.Abs(".")
	r, err := git.PlainOpen(repositoryPath)
	if err != nil {
		println("打开仓库失败")
		return
	}
	w, err := r.Worktree()
	if err != nil {
		println("打开仓库失败")
		println(err)
		return
	}

	_, err = w.Add(".")
	if err != nil {
		println("向仓库添加文件失败")
		println(err)
		return
	}
	statue, _ := w.Status()
	println(statue.String())

	commit, err := w.Commit("example go-git commit", &git.CommitOptions{
		Author: &object.Signature{
			Name: "Wang",
			When: time.Now(),
		},
	})

	obj, _ := r.CommitObject(commit)
	println("提交文件：")
	println(obj)

	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
	})
	if err != nil {
		println("上传失败")
		println(err)
		return
	}
}

func main() {
	//filePath := "E:/desktop"
	//blogList := getBlogList(filePath)
	//for _, blog := range blogList {
	//	extractPicture(&blog)
	//	println(blog.name, blog.pictures, blog.directoryPath, blog.targetPath)
	//	println(blog.pictures[0].pictureName, blog.pictures[0].targetName)
	//	println(blog.pictures[1].pictureName, blog.pictures[1].targetName)
	//	println(blog.pictures[2].pictureName, blog.pictures[2].targetName)
	//	copyBlog(blog)
	//	copyPicture(blog)
	//}

	gitOperate()
}
