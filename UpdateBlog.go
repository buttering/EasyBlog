package main

import "EasyBlogs/tools"

// UpdateBlogs 更新已整理的博客
// 1. 根据hash名找到博客（位于已整理区域）
// 2. 重新处理图片url
// 3. 提交更新
func UpdateBlogs() {
	yamlContent := tools.YamlReader(YAML_FILE_PATH)
}
