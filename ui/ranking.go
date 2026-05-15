package ui

import (
	"fmt"
	"time"

	"eurovote/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type draggableRow struct {
	widget.BaseWidget
	content          fyne.CanvasObject
	labelText        string
	index            int
	list             *[]models.Act
	refresh          func()
	refreshHighlight func()
	findTarget       func(absY float32) int
	window           fyne.Window
	ghost            *widget.PopUp
	lastTarget       int
	sharedHighlight  *int
	highlightBottom  *bool
}

type draggableRowRenderer struct {
	row      *draggableRow
	line     *canvas.Line
	objects  []fyne.CanvasObject
	lastSize fyne.Size
}

func (r *draggableRowRenderer) Layout(size fyne.Size) {
	r.lastSize = size
	r.row.content.Resize(size)
	r.row.content.Move(fyne.NewPos(0, 0))
}

func (r *draggableRowRenderer) MinSize() fyne.Size {
	return r.row.content.MinSize()
}

func (r *draggableRowRenderer) Refresh() {
	if *r.row.sharedHighlight == r.row.index {
		r.line.Show()
		w := r.lastSize.Width
		if *r.row.highlightBottom {
			y := r.lastSize.Height - 1
			r.line.Position1 = fyne.NewPos(0, y)
			r.line.Position2 = fyne.NewPos(w, y)
		} else {
			r.line.Position1 = fyne.NewPos(0, 1)
			r.line.Position2 = fyne.NewPos(w, 1)
		}
	} else {
		r.line.Hide()
	}
	r.line.Refresh()
	canvas.Refresh(r.row.content)
}

func (r *draggableRowRenderer) Destroy() {}

func (r *draggableRowRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func newDraggableRow(content fyne.CanvasObject, labelText string, index int, list *[]models.Act, sharedHighlight *int, highlightBottom *bool, w fyne.Window, refresh, refreshHighlight func(), findTarget func(float32) int) *draggableRow {
	r := &draggableRow{
		content:          content,
		labelText:        labelText,
		index:            index,
		list:             list,
		sharedHighlight:  sharedHighlight,
		highlightBottom:  highlightBottom,
		window:           w,
		refresh:          refresh,
		refreshHighlight: refreshHighlight,
		findTarget:       findTarget,
		lastTarget:       index,
	}
	r.ExtendBaseWidget(r)
	return r
}

func (r *draggableRow) CreateRenderer() fyne.WidgetRenderer {
	line := canvas.NewLine(theme.PrimaryColor())
	line.StrokeWidth = 3
	line.Hide()
	return &draggableRowRenderer{
		row:     r,
		line:    line,
		objects: []fyne.CanvasObject{r.content, line},
	}
}

func (r *draggableRow) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (r *draggableRow) Dragged(ev *fyne.DragEvent) {
	if r.ghost == nil {
		r.ghost = widget.NewPopUp(widget.NewLabel(r.labelText), r.window.Canvas())
	}
	pos := ev.AbsolutePosition
	pos.X += 12
	pos.Y += 8
	r.ghost.ShowAtPosition(pos)

	target := r.findTarget(ev.AbsolutePosition.Y)
	r.lastTarget = target

	atBottom := target > r.index
	if *r.sharedHighlight != target || *r.highlightBottom != atBottom {
		*r.sharedHighlight = target
		*r.highlightBottom = atBottom
		r.refreshHighlight()
	}
}

func (r *draggableRow) DragEnd() {
	if r.ghost != nil {
		r.ghost.Hide()
		r.ghost = nil
	}

	to := r.lastTarget
	*r.sharedHighlight = -1
	r.refreshHighlight()

	from := r.index
	if from == to {
		return
	}

	l := *r.list
	if to < 0 {
		to = 0
	}
	if to >= len(l) {
		to = len(l) - 1
	}

	item := l[from]
	without := make([]models.Act, 0, len(l)-1)
	for i, a := range l {
		if i != from {
			without = append(without, a)
		}
	}
	result := make([]models.Act, len(l))
	copy(result[:to], without[:to])
	result[to] = item
	copy(result[to+1:], without[to:])

	*r.list = result
	r.refresh()
}

// makeRowFinder returns a function that maps an absolute Y canvas position to
// the index of the row it falls within, using each row's real rendered bounds.
func makeRowFinder(drRows *[]*draggableRow) func(float32) int {
	return func(absY float32) int {
		rows := *drRows
		driver := fyne.CurrentApp().Driver()
		for i, dr := range rows {
			rowPos := driver.AbsolutePositionForObject(dr)
			if absY < rowPos.Y+dr.Size().Height {
				return i
			}
		}
		return len(rows) - 1
	}
}

// NewRankingScreen shows all acts in a drag-to-reorder list.
func NewRankingScreen(acts []models.Act, w fyne.Window) fyne.CanvasObject {
	list := make([]models.Act, len(acts))
	copy(list, acts)

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Eurovision %d — All Acts", currentYear()),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	subtitle := widget.NewLabel("Drag acts to rank them in your preferred order.")
	subtitle.Alignment = fyne.TextAlignCenter

	rows := container.NewVBox()
	highlightIdx := -1
	highlightBottom := false
	var drRows []*draggableRow
	findTarget := makeRowFinder(&drRows)

	refreshHighlight := func() {
		for _, dr := range drRows {
			dr.Refresh()
		}
	}

	var refresh func()
	refresh = func() {
		rows.Objects = nil
		drRows = nil
		for i, act := range list {
			i, act := i, act
			labelText := fmt.Sprintf("%s: %s - %s", act.Country, act.Artist, act.Song)
			handle := widget.NewLabel("≡")
			label := widget.NewLabel(fmt.Sprintf("%d.  %s", i+1, labelText))
			label.Wrapping = fyne.TextWrapWord
			row := container.NewBorder(nil, nil, handle, nil, label)
			dr := newDraggableRow(row, labelText, i, &list, &highlightIdx, &highlightBottom, w, refresh, refreshHighlight, findTarget)
			drRows = append(drRows, dr)
			rows.Add(dr)
		}
		rows.Refresh()
	}

	refresh()

	scroll := container.NewVScroll(rows)
	header := container.NewVBox(title, subtitle, widget.NewSeparator())
	return container.NewBorder(header, nil, nil, nil, scroll)
}

func currentYear() int {
	return time.Now().Year()
}
