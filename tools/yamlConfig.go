package tools

import (
	"gopkg.in/yaml.v3"
	"os"
)

//type Blog struct {
//	Name       string   `yaml:"name"` // 如果首字母小写，是不可导出的，无法被 yaml 包正确解析。yaml 包只会解析可导出的字段。
//	CreateDate string   `yaml:"createDate"`
//	UpdateDate string   `yaml:"updateDate"`
//	Target     []string `yaml:"target"`
//}

type BlogList struct {
	Blogs []Blog `yaml:"blogs"`
}

type Blog struct {
	Name string `yaml:"name"`
	Hash string `yaml:"hash"`
}

func YamlReader(filePath string) BlogList {
	var blogs BlogList
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		println(err.Error())
		panic(err)
	}

	err = yaml.Unmarshal(yamlFile, &blogs)
	if err != nil {
		println(err.Error())
		panic(err)
	}

	return blogs
}

func YamlWriter(filePath string, blog *BlogList) {
	yamlData, err := yaml.Marshal(&blog)
	if err != nil {
		println(err.Error())
		panic(err)
	}

	err = os.WriteFile(filePath, yamlData, 0644)
	if err != nil {
		println(err.Error())
		panic(err)
	}
}
