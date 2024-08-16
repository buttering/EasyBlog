package main

import (
	"EasyBlogs/tools"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type markdownPicture struct {
	isUrl             bool
	sourcePicturePath string
	start             int // md中源图片路径的起始偏移量
	end               int
	hashName          string // 均将包含后缀的文件名进行hash，且后拼接上原有后缀名
	targetUrl         string // 修改后在github仓库中的url
}

type Blog struct {
	name          string
	hashName      string
	pictures      []markdownPicture
	directoryPath string // 源文件文件夹路径
	legal         bool   // 成功通过解析
}

func getBlogList() (blogsList []Blog) {
	blogsList = make([]Blog, 0, 10)

	fileList, err := os.ReadDir(LOCAL_FILE_PATH)
	if err != nil {
		panic(err)
	}

	for _, file := range fileList {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".md" {
			fileName := file.Name()

			blogsList = append(blogsList, Blog{fileName, tools.Hash(fileName), nil, LOCAL_FILE_PATH, false})
		}
	}
	return
}

func ExtractPicture(blog *Blog) {
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
		var pictureName string
		if isUrl(picturePath) {
			u, err := url.Parse(picturePath)
			if err != nil {
				println("解析图片url：", picturePath, " 失败")
				continue
			}
			pictureName = path.Base(u.Path)
		} else if filepath.IsAbs(picturePath) {
			pictureName = filepath.Base(picturePath)
		} else { // 相对路径的本地文件
			picturePath = filepath.Join(blog.directoryPath, picturePath)
			pictureName = filepath.Base(picturePath)
		}
		hashName := tools.Hash(pictureName) + path.Ext(pictureName)

		blog.pictures = append(
			blog.pictures,
			markdownPicture{
				isUrl(picturePath),
				picturePath,
				start,
				end,
				hashName,
				REPOSITORY_URL + "/" + blog.hashName + "/" + hashName, // 使用path.join会导致'https://'后的双斜杠变为单斜杠
			},
		)
	}

	blog.legal = true
}

func copyBlog(blog *Blog) {
	fmt.Println("拷贝博客：“" + blog.name + "”")

	blogTargetPath := filepath.Join(BLOG_PATH, blog.name)
	pictureTargetPath := filepath.Join(PICTURE_PATH, blog.hashName)
	if _, err := os.Stat(blogTargetPath); !os.IsNotExist(err) {
		println("文章“" + blog.name + "”已经存在")
		blog.legal = false
		return
	}

	if err := os.Mkdir(pictureTargetPath, 0777); err != nil {
		println("为博客“" + blog.name + "”创建对应picture文件夹失败")
		blog.legal = false
		return
	}

	content, _ := os.ReadFile(filepath.Join(blog.directoryPath, blog.name))

	offset := 0
	for _, picture := range blog.pictures {
		start := picture.start + offset
		end := picture.end + offset
		content = append(content[:start], append([]byte(picture.targetUrl), content[end:]...)...)
		offset += len(picture.targetUrl) - (end - start)
	}

	err := os.WriteFile(blogTargetPath, content, 0644)
	if err != nil {
		println("复制文件“" + blog.name + "”错误")
		blog.legal = false
	}

}

func CopyPicture(blog Blog) {
	pictureTargetPath := filepath.Join(PICTURE_PATH, blog.hashName)

	for _, picture := range blog.pictures {
		fmt.Println("导入图片：“" + picture.sourcePicturePath + "”")

		var sourceFile interface{}
		if picture.isUrl {
			for i := 0; i < 5; i++ {
				response, err := http.Get(picture.sourcePicturePath)
				if err == nil && response.StatusCode == http.StatusOK {
					sourceFile = response.Body
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			if sourceFile == nil {
				println("下载图片“" + picture.sourcePicturePath + "”失败")
				continue
			}

		} else {
			file, err := os.Open(picture.sourcePicturePath)
			if err != nil {
				println("打开图片“" + picture.sourcePicturePath + "”失败")
				continue
			}
			sourceFile = file
		}

		destinationFile, _ := os.Create(filepath.Join(pictureTargetPath, picture.hashName))
		_, err := io.Copy(destinationFile, sourceFile.(io.Reader))
		if err != nil {
			println("复制图片“" + picture.sourcePicturePath + "”失败")
		}
	}
}

// yaml文件记录文章和对应的hash字符串，用于管理文章（增删改）
func yamlOperate(blogList []Blog) {
	fmt.Println("追加yaml文件")
	yamlContent := tools.YamlReader(YAML_FILE_PATH)
	// 不变更已有的，只追加
	for _, blog := range blogList {
		if !blog.legal {
			continue
		}
		yamlContent.Blogs = append(yamlContent.Blogs, tools.Blog{
			Name: blog.name,
			Hash: tools.Hash(blog.name),
		})
	}
	tools.YamlWriter(YAML_FILE_PATH, &yamlContent)
}

func dbOperate(blogList []Blog) {
	fmt.Println("导入数据库")
	db := tools.GetConnection()
	defer db.Close()
	for _, blog := range blogList {
		if !blog.legal {
			continue
		}
		now := time.Now().Format("2006-01-02")
		_, err := db.Exec(tools.InsertBlog, blog.name, tools.Published, now, now, 0, 0, "Wang Jiawei")
		if err != nil {
			log.Fatal(err)
		}
	}
}

func GitOperate(blogList []Blog, sprintf func([]string) string) {
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

	_, err = w.Add("./asset")
	if err != nil {
		println("向仓库添加文件失败")
		println(err.Error())
		return
	}
	status, _ := w.Status()
	println("git 状态：")
	println(status.String())

	nameList := tools.Map(blogList, func(blog Blog) string {
		return blog.name
	})
	summary := sprintf(nameList)
	commit, err := w.Commit(summary, &git.CommitOptions{
		Author: &object.Signature{
			Name: "Wang",
			When: time.Now(),
		},
	})

	obj, _ := r.CommitObject(commit)
	fmt.Println("提交文件：")
	fmt.Println(obj.String())

	// user必须是"git"。。。困扰了半天，最后查issue发现的。真够郁闷的。
	privateKey, err := ssh.NewPublicKeysFromFile("git", "./resource/githubPublicKey", "")

	if err != nil {
		println(err.Error())
	}

	for i := 1; i <= 3; i++ {
		err = r.Push(&git.PushOptions{
			RemoteName: "origin",
			RemoteURL:  `git@github.com:buttering/EasyBlogs.git`,
			Auth:       privateKey,
			Progress:   os.Stdout,
		})
		if err == nil {
			break
		}
		println("第 ", i, " 次上传失败, 失败原因：", err.Error())
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		println("重试次数已达上限，上传失败")
		return
	}

	fmt.Println("提交成功！")
}

// UploadBlogs 整理并提交新博客
// 1. 预处理文章
// 2. 将博客和图片放入asset文件夹
// 3. 追加yaml文件 （TODO）
// 4. git提交触发CI/CD流程

func UploadBlogs() {
	blogList := getBlogList()
	for i := range blogList {
		ExtractPicture(&blogList[i])
		copyBlog(&blogList[i])
		CopyPicture(blogList[i])
	}
	if len(blogList) == 0 {
		return
	}

	yamlOperate(blogList)
	// 改用github page进行博客部署，不需要额外记录博客信息
	//dbOperate(blogList)
	GitOperate(blogList, func(nameList []string) (summary string) {
		if len(nameList) == 1 {
			summary = fmt.Sprintf("提交文件 [%s]", blogList[0].name)
		} else {
			summary = fmt.Sprintf(
				"提交 %d 个博客\n"+
					"\n"+
					"文件列表: [%s]",
				len(blogList),
				strings.Join(nameList, ", "),
			)
		}
		return
	})

}
