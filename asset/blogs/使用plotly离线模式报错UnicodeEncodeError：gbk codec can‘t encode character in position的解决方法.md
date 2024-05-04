---
title: 使用plotly离线模式报错‘UnicodeEncodeError: ‘gbk‘ codec can‘t encode character in position: ‘的解决方法
date: 2022-10-15 17:51:24
toc: true
mathjax: true
tags:
- 杂项
- plotly
- 编码
- gbk
---

# 问题
使用plotly离线模式绘制图像时,报错:

```powershell
UnicodeEncodeError: 'gbk' codec can't encode character '\u25c4' in position 276398: illegal multibyte sequence
```
![报错描述](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/bdf03a9b9bdacaaf60d1f899c0222865/74d51bfa109b4065ccb3ba37b6922bb3.png)
# 环境
IDE: PyCharm 2022.3.1
操作系统: Windows 10
Python版本: 3.11

**错误相关项目代码**
```python
plot_data = [plot_training_trace, plot_test_trace]
plot_figure = go.Figure(layout=plot_layout, data=plot_data)
pyoff.plot(plot_figure)  # 这里报错
```
# 问题查找过程
尝试使用如[修改IDE编码](https://blog.csdn.net/lj606/article/details/121752437)的办法未果。
通过debug，发现是```pathlib.py```文件中的```write_text```函数未能正确将文件按照utf-8进行编码。
![write_text函数debug](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/bdf03a9b9bdacaaf60d1f899c0222865/1ed4e79576fd90153117531cab86df90.png)
尝试手动修改encoding值，再次运行发现能够正确绘制网页
![手动修改encoding](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/bdf03a9b9bdacaaf60d1f899c0222865/0ec2f81a958f960094bafc7d88c71415.png)
证明```io.text_encoding()```返回的确为'locale'。查看windows默认编码，发现为GBK
![windows编码](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/bdf03a9b9bdacaaf60d1f899c0222865/ac4ddef02e4ff6c9ae8cf499176edd0d.png)
# 解决办法
基于此，有两个解决方法：
1. 修改plotly对```write_text```的调用
编辑```_html.py```（位于项目的site-packages中，如```C:\Users\38412\.virtualenvs\machine-learning-TGwdIfnC\Lib\site-packages\plotly\io\_html.py```），修改函数```write_html```如图，为```write_text()```添加encodeing参数为'utf8'。![修改_html.py](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/bdf03a9b9bdacaaf60d1f899c0222865/89e1637966679d4832fd25d2d56ed3de.png)
2. 修改操作系统的默认编码集
[详见参考链接](https://zhuanlan.zhihu.com/p/153219931#:~:text=%E8%AE%BE%E7%BD%AE%E6%96%B9%E6%B3%95%EF%BC%9A%E6%8E%A7%E5%88%B6%E9%9D%A2%E6%9D%BF-%3E%E5%8C%BA%E5%9F%9F-%3E%E7%AE%A1%E7%90%86%3E%E6%9B%B4%E6%94%B9%E7%B3%BB%E7%BB%9F%E5%8C%BA%E5%9F%9F%E8%AE%BE%E7%BD%AE,%E8%AE%BE%E7%BD%AE%E5%A5%BD%E5%90%8E%EF%BC%8C%E9%87%8D%E5%90%AF%EF%BC%8C%E7%B3%BB%E7%BB%9F%E7%BC%96%E7%A0%81%E5%8D%B3%E5%8F%98%E4%B8%BAUTF-8%E6%A0%BC%E5%BC%8F%E3%80%82)，将系统编码修改为utf-8。
![修改操作系统编码集](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/bdf03a9b9bdacaaf60d1f899c0222865/720674378cbd8f77db886ac893f5467d.png)

**2023.3.9更新：**
**使用Unicode UTF-8会导致WSL子系统无法打开explorer.exe**