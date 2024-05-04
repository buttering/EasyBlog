---
title: lvgl修改图片大小上限
date: 2024-01-09 18:43:06
toc: true
mathjax: true
tags:
- lvgl
- GUI
- 嵌入式
---

## 问题描述
在lvgl中读取图片文件时，被读取的图片具有上限，也就是2048像素。这会造成两个非预期的结果：
1. 超过2048像素的部分会被裁去。
2. 表示图片的结构体`lv_img_t`中的`w`和`h`变量值是图片像素被2048求余。例如，当一个图片高为2048像素时，`h`的值被赋值为1。此时如果使用`lv_img_set_offset_y`函数修改图片偏移量，lvgl会以1作为图片高度进行偏移量的计算。
## 解决办法
解决办法是修改项目目录下的`./lvgl/src/draw/lv_img_buf.h`文件中的`lv_img_header_t`结构体。其中的`w`和`h`成员限制了图片的上限。图片的高宽上限分别为$2^h$和$2^w$。

例如将w和h修改为13时：
```c
typedef struct {

    uint32_t cf : 5;          /*Color format: See `lv_img_color_format_t`*/
    uint32_t always_zero : 3; /*It the upper bits of the first byte. Always zero to look like a
                                 non-printable character*/

    uint32_t reserved : 2; /*Reserved to be used later*/

    uint32_t w : 13; /*Width of the image map*/
    uint32_t h : 13; /*Height of the image map*/
} lv_img_header_t;
```
此时图片高宽上限为8192像素。