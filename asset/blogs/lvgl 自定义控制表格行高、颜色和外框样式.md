---
title: lvgl 自定义控制表格行高、颜色和外框样式
date: 2023-07-24 10:18:59
toc: true
mathjax: true
categories:
- LVGL
tags:
- lvgl
- GUI
- 嵌入式
---

# lvgl 自定义控制表格行高、颜色和外框样式

lvgl版本：8.3.7
lvgl自带表格控件能够指定列宽，但是表格行高是根据内容动态渲染的。

表格自带样式如图，带有蓝色的外框和白底。
![在这里插入图片描述](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/b46077f88be412624886df2717b576be/56320abf44c3ede843a75010f28584b8.png)

如果想要手动控制表格行高、颜色和外框等属性，需要监听表格绘制事件，在事件中覆盖属性。下面给出相关代码：

```c
#include "table_oper.h"

 // 点击事件监听函数
void tbl_clicked_style_handler(lv_event_t *e) {
    lv_obj_t *table = lv_event_get_target(e);
    Style *style = lv_event_get_user_data(e);

    Common_Style *common_style = style->common_style;
    Table_Style *exclusive_style = style->exclusive_style;

    // 读取被点击的单元格，存到变量中
    lv_table_get_selected_cell(table, &exclusive_style->click_row, &exclusive_style->click_col);
}

// 单元格的通用样式
void cell_draw(lv_obj_draw_part_dsc_t* dsc, uint32_t row, uint32_t col, Style *style) {
    Common_Style *common_style = style->common_style;
    Table_Style *exclusive_style = style->exclusive_style;

    dsc->label_dsc->color = lv_color_hex(exclusive_style->row_font_color[row]);
    dsc->rect_dsc->bg_color = lv_color_hex(exclusive_style->row_background_color[row]);

    dsc->rect_dsc->outline_color = lv_color_hex(exclusive_style->outline_color);
    dsc->rect_dsc->outline_width = exclusive_style->outline_width;
}

// 被点击时的单元格样式，分为行选中、列选中和单元格选中
void cell_clicked_draw(lv_obj_draw_part_dsc_t* dsc, uint32_t row, uint32_t col, Table_Style *style) {
    switch (style->select_mode)
    {
    case 0:
        if (row == style->click_row)
            dsc->rect_dsc->bg_color = lv_color_hex(style->select_color);
        break;
    case 1:
        if (col == style->click_col)
            dsc->rect_dsc->bg_color = lv_color_hex(style->select_color);
        break;
    case 2:
        if (row == style->click_row && col == style->click_col)
            dsc->rect_dsc->bg_color = lv_color_hex(style->select_color);
        break;
    }
}

void table_draw(lv_obj_draw_part_dsc_t *dsc, Style *style){
    dsc->rect_dsc->bg_opa = 0;  // 隐藏最下层的白框
}

// 重绘事件监听函数
void tbl_draw_style_handler(lv_event_t *e) {
    lv_obj_t *widget = lv_event_get_target(e);
    lv_table_t *table = (lv_table_t *)widget;
    lv_obj_draw_part_dsc_t *dsc = lv_event_get_param(e);  // 获取结构体
    Style *style = lv_event_get_user_data(e);

    Common_Style *common_style = style->common_style;
    Table_Style *exclusive_style = style->exclusive_style;

    // 控制单元格样式
    if (dsc != NULL && dsc->part == LV_PART_ITEMS){
        uint32_t row = dsc->id /  lv_table_get_col_cnt(widget);
        uint32_t col = dsc->id - row * lv_table_get_col_cnt(widget);

        cell_draw(dsc, row, col, style);
        cell_clicked_draw(dsc, row, col, exclusive_style);
    }

    // 控制外框线和白底
    if (dsc != NULL && dsc->part == LV_PART_MAIN){
        table_draw(dsc, style);
    }

    // 控制行高
    for (size_t row = 0; row < exclusive_style->row_num; row++)
    {
        table->row_h[row] = exclusive_style->row_height;
    }
}

// 入口
lv_obj_t* add_table(Style *style){
    lv_obj_t *table;

    Common_Style *common_style = style->common_style;
    Table_Style *exclusive_style = style->exclusive_style;

    table = lv_table_create(lv_scr_act());
    
    // 整体样式，省略字体文字等代码
    lv_obj_set_x(table, common_style->h_ofs);
    lv_obj_set_y(table, common_style->v_ofs);

    // 表格样式
    lv_table_set_row_cnt(table, exclusive_style->row_num);
    lv_table_set_col_cnt(table, exclusive_style->col_num);
    for (size_t col = 0; col < exclusive_style->col_num; col++)
    {
        // 列宽
        lv_table_set_col_width(table, col, exclusive_style->col_width[col]);
    }

    // 重绘事件,控制单元格和行样式
    lv_obj_add_event_cb(table, tbl_draw_style_handler, LV_EVENT_DRAW_PART_BEGIN, style);
	// 表格的点击应该监听 LV_EVENT_VALUE_CHANGED 事件
    lv_obj_add_event_cb(table, tbl_clicked_style_handler, LV_EVENT_VALUE_CHANGED, style);

    return table;
}

```

因为是实际项目，使用了较为复杂的结构体进行样式的控制，style结构体定义如下，common_style和exclusicec_style的具体属性就不贴出来了，就是一些字符串和整型数值：

```c
typedef struct Style {

    struct Common_Style *common_style;  // 通用样式

    void *exclusive_style;  // 控件专有样式

    lv_obj_t* widget;  // 控件对象

} Style;
```

## 具体步骤：

1. 监听`LV_EVENT_DRAW_PART_BEGIN`事件。
2. 在事件监听函数中获取`lv_obj_draw_part_dsc_t`结构体。
3. 结构体属性赋值。

## 效果
![在这里插入图片描述](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/b46077f88be412624886df2717b576be/70e860235860bb68b0d85bfdf622e035.png)
去除了外框线和白底，自定义行高。
![在这里插入图片描述](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/b46077f88be412624886df2717b576be/6550f649c241373984871022c66befe6.png)


根据配置实现了点击行选中的颜色更改。