package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// AboutLink contains one action shown on the About page.
type AboutLink struct {
	ID    string
	Label string
	Icon  *woxui.Image
	OnTap func()
}

// AboutSettingsProps contains the About branding, running version, and actions.
type AboutSettingsProps struct {
	Width       float32
	Height      float32
	AppIcon     *woxui.Image
	Version     string
	Description string
	Status      string
	Links       []AboutLink
	Theme       woxcomponent.ControlTheme
}

// AboutSettingsView builds the About settings route.
func AboutSettingsView(props AboutSettingsProps) woxwidget.Widget {
	contentWidth := min(float32(600), max(float32(0), props.Width-48))
	links := make([]woxwidget.Widget, 0, len(props.Links))
	for _, link := range props.Links {
		links = append(links, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: link.ID, Label: link.Label, Icon: link.Icon, IconSize: 18, IconGap: 8, IntrinsicWidth: true,
			Variant: woxcomponent.ButtonText, Padding: woxwidget.Insets{Left: 6, Right: 6}, OnTap: link.OnTap, Theme: props.Theme,
		}))
	}

	var logo woxwidget.Widget = woxwidget.Container{
		Width: 80, Height: 80, Radius: 18, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255},
		Child: woxwidget.Align{Width: 80, Height: 80, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: "W", Style: woxui.TextStyle{Size: props.Theme.Scaled(48), Weight: woxui.FontWeightSemibold}, Color: woxui.Color{A: 255},
		}},
	}
	if props.AppIcon != nil {
		logo = woxwidget.Image{Source: props.AppIcon, Width: 80, Height: 80}
	}

	children := []woxwidget.Widget{
		woxwidget.Container{Height: 20},
		woxwidget.Align{Width: contentWidth, Height: 80, Horizontal: 0.5, Child: logo},
		woxwidget.Container{Height: 20},
		woxwidget.Align{Width: contentWidth, Height: 28, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{
			Height: 26, Radius: 16, Color: props.Theme.Accent, Padding: woxwidget.Insets{Left: 12, Top: 4, Right: 12, Bottom: 4},
			Child: woxwidget.Text{Value: props.Version, Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: props.Theme.AccentText},
		}},
		woxwidget.Container{Height: 20},
		woxwidget.Align{Width: contentWidth, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: props.Description, Style: woxui.TextStyle{Size: props.Theme.Scaled(16)}, Color: props.Theme.Text,
		}},
		woxwidget.Container{Height: 28},
		woxwidget.Align{Width: contentWidth, Height: 32, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 30, Children: links}},
	}

	// kuankuanlv 自建区块（v3）：横线分隔 + 版本/仓库信息 + 改动项竖排文字列表
	type aboutLine struct {
		text    string
		heading bool
		gap     bool
	}
	aboutLines := []aboutLine{
		{text: "基于原仓库 Wox 修改（GPL-3.0）", heading: true},
		{gap: true},
		{text: "基于版本：v2.4.5"},
		{text: "自定义版本：v2.4.5-kuankuanlv.4"},
		{text: "原仓库：https://github.com/Wox-launcher/Wox"},
		{text: "fork仓库：https://github.com/kuankuanlv/Wox"},
		{gap: true},
		{text: "改动项：", heading: true},
		{text: "1. 快捷键", heading: true},
		{text: "   · 注册制重构：全局/应用内分层，均可自定义"},
		{text: "   · 新增 cmd+q 退出、cmd+w 关闭窗口"},
		{text: "2. 主搜索流程", heading: true},
		{text: "   · 四相位重构：先筛后算，kw 级自学习排序"},
		{text: "   · Tab 路径层级补全"},
		{text: "   · 结果按插件分组"},
		{text: "   · 自学习偏好收敛 kw 维度（详情变化不影响排序）"},
		{text: "3. 插件系统瘦身", heading: true},
		{text: "   · 内置插件外部化，core 更轻（脚本插件承接）"},
		{text: "   · 修复插件设置表格行序错位（防配置误写）"},
		{text: "4. 插件系统扩展", heading: true},
		{text: "   · kuankuanlv 扩展：InputFilter 准入、参数提示、输入输出说明"},
		{text: "   · 单文件插件动态注册能力"},
		{text: "另：自动更新默认关闭"},
	}
	changes := []woxwidget.Widget{
		woxwidget.Container{Height: 16},
		woxwidget.Container{Width: contentWidth, Height: 1, Color: props.Theme.TextSecondary},
		woxwidget.Container{Height: 12},
	}
	for _, ln := range aboutLines {
		if ln.gap {
			changes = append(changes, woxwidget.Container{Height: 8})
			continue
		}
		style := woxui.TextStyle{Size: props.Theme.Scaled(12)}
		color := props.Theme.TextSecondary
		if ln.heading {
			style = woxui.TextStyle{Size: props.Theme.Scaled(13), Weight: woxui.FontWeightSemibold}
			color = props.Theme.Text
		}
		changes = append(changes, woxwidget.Align{Width: contentWidth, Height: 19, Vertical: 0, Child: woxwidget.Text{
			Value: ln.text, Style: style, Color: color,
		}})
	}
	children = append(children, changes...)
	if props.Status != "" {
		children = append(children,
			woxwidget.Container{Height: 18},
			woxwidget.Align{Width: contentWidth, Height: 18, Horizontal: 0.5, Child: woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.Theme.Error}},
		)
	}
	children = append(children, woxwidget.Container{Height: 40})

	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Align{
		Width: props.Width, Height: props.Height, Horizontal: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}}
}
