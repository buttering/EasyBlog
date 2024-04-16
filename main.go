package main

import (
	"EasyBlogs/tools"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
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
	legal         bool   // 成功通过解析
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
			blogsList = append(blogsList, Blog{fileName, nil, path, targetPath, false})
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

	blog.legal = true
}

func copyBlog(blog *Blog) {
	println("拷贝博客：“" + blog.name + "”")

	if _, err := os.Stat(blog.targetPath); !os.IsNotExist(err) {
		println("文章“" + blog.name + "”已经存在")
		blog.legal = false
		return
	}

	if err := os.Mkdir(blog.targetPath, 0777); err != nil {
		println("创建文件夹“" + blog.name + "”失败")
		blog.legal = false
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
		blog.legal = false
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

func yamlOperate(yamlPath string, blogList []Blog) {
	println("生成yaml文件")
	yamlStruct := tools.YamlReader(yamlPath)
	// 不变更已有的，只追加
	for _, blog := range blogList {
		if !blog.legal {
			continue
		}
		yamlStruct.Blogs = append(yamlStruct.Blogs, tools.Blog{
			Name:       blog.name,
			CreateDate: time.Now().Format("2003-01-02"),
			UpdateDate: time.Now().Format("2003-01-02"),
		})
	}
	tools.YamlWriter(yamlPath, &yamlStruct)

}

func gitOperate(blogList []Blog) {
	if len(blogList) == 0 {
		return
	}
	repositoryPath, _ := filepath.Abs(".")
	r, err := git.PlainOpen(repositoryPath)
	if err != nil {
		println("打开仓库失败")
		return
	}
	w, err := r.Worktree()
	if err != nil {
		println("打开仓库失败")
		println(err.Error())
		return
	}

	_, err = w.Add("./resource")
	if err != nil {
		println("向仓库添加文件失败")
		println(err.Error())
		return
	}
	status, _ := w.Status()
	println(status.String())

	nameList := tools.Map(blogList, func(blog Blog) string {
		return blog.name
	})
	var summary string
	if len(nameList) == 1 {
		summary = fmt.Sprintf("提交文件 [%s]", blogList[0].name)
	} else {
		summary = fmt.Sprintf(
			"提交 %d 个文件\n"+
				"\n"+
				"文件列表: [%s]",
			len(blogList),
			strings.Join(nameList, ", "),
		)
	}
	commit, err := w.Commit(summary, &git.CommitOptions{
		Author: &object.Signature{
			Name: "Wang",
			When: time.Now(),
		},
	})

	obj, _ := r.CommitObject(commit)
	println("提交文件：")
	println(obj.String())

	// user必须是"git"。。。困扰了半天，最后查issue发现的。真够郁闷的。
	privateKey, err := ssh.NewPublicKeysFromFile("git", "./githubPublicKey", "")

	if err != nil {
		println(err.Error())
	}

	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		RemoteURL:  `git@github.com:buttering/EasyBlogs.git`,
		Auth:       privateKey,
		Progress:   os.Stdout,
	})
	if err != nil {
		println("上传失败")
		println(err.Error())
		return
	}

	println("提交成功！")
}

func main() {
	filePath := "E:/desktop"
	yamlPath := "./resource/blogs-list.yaml"
	blogList := getBlogList(filePath)
	for i := range blogList {
		extractPicture(&blogList[i])
		//println(blog.name, blog.pictures, blog.directoryPath, blog.targetPath)
		//println(blog.pictures[0].pictureName, blog.pictures[0].targetName)
		//println(blog.pictures[1].pictureName, blog.pictures[1].targetName)
		//println(blog.pictures[2].pictureName, blog.pictures[2].targetName)
		copyBlog(&blogList[i])
		copyPicture(blogList[i])
	}
	if len(blogList) == 0 {
		return
	}

	yamlOperate(yamlPath, blogList)
	//gitOperate(blogList)

}
