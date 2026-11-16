package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// backdropButton names the effect of the backdrop, and opens the menu of
// the others. It sits at the top of Now Playing, in every pane, so that the
// background can be changed from beside what plays, as the stage's style can.
func (a *app) backdropButton(c *ui.Context) {
	m := ui.MenuBase(c.Key("backdrop"), &a.backdropOpen)
	b := m.Trigger.Padding(6, 10, 6, 12).Shrink(0).Radius(999).Gap(4).AlignItems(ui.Center).
		FocusRing(false).Label("背景效果").Tooltip("背景效果")
	if w, _ := c.Size(); w < 1000 {
		b.Padding(6, 8)
	}
	mediaFeedback(b, radixWhite(8))
	angle := float32(0)
	if a.backdropOpen {
		b.Background(radixWhite(2)).TextColor(radixPaper)
		angle = 180
	}
	b.Children(func() {
		ui.Text(c, "背景").FontSize(13).FontWeight(600)
		ui.Icon(c, iconDown).Size(12, 12).Rotate(b.AnimateTo("arrow", angle, ui.Animation{Duration: 140 * time.Millisecond}))
	})
	m.Popup(func(panel ui.Element) {
		// The player is a dark media surface even when the pages use the light theme.
		panel.AttachTo(b, ui.AnchorBottomEnd, ui.AnchorTopEnd).Margin(6, 0, 0, 0).
			Width(160).Padding(6).Gap(2).Radius(12).Background(radixSage(true, 2)).
			Border(0.5, radixSage(true, 6)).Shadow(0, 8, 24, -4, radixBlack(6)).Label("背景效果菜单")
		for _, style := range backdropStyles {
			selected := a.settings.backdropStyle() == style.id
			item := stageMenuRow(m.RadioItem(selected), selected).Label(style.label)
			item.Children(func() {
				ui.Text(c, style.label).FontSize(13).Grow(1)
				ui.Box(c).Size(16, 16).Children(func() {
					if selected {
						ui.Icon(c, iconCheck).Size(16, 16).TextColor(radixGreen(true, 11))
					}
				})
			})
			if item.Clicked() && !selected {
				a.settings.setBackdropStyle(style.id)
				a.settings.save()
				a.redraw()
			}
		}
	})
}
