package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
)

type PageView struct {
	Categories    []CategoryView
	ActiveDetails template.HTML // Pre-rendered details for deep linking
	OOB           bool          // Always false for full page renders
}

type DeleteOOBView struct {
	ID string
}

func (p *Presentation) RenderIndex(w io.Writer, categories []CategoryView) error {
	return p.RenderIndexWithDetails(w, categories, nil)
}

func (p *Presentation) RenderIndexWithDetails(w io.Writer, categories []CategoryView, detailsView any) error {
	pageView := PageView{Categories: categories}

	if detailsView != nil {
		var buf bytes.Buffer

		switch v := detailsView.(type) {
		case TaskView:
			if err := p.tmpl.ExecuteTemplate(&buf, "details", v); err != nil {
				return err
			}
		case SubtaskView:
			if err := p.tmpl.ExecuteTemplate(&buf, "subtask_details", v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown details view type: %T", v)
		}

		pageView.ActiveDetails = template.HTML(buf.String())
	}

	return p.tmpl.ExecuteTemplate(w, "layout.html", pageView)
}

func (p *Presentation) RenderSlideoverClear(w io.Writer) error {
	view := PageView{
		ActiveDetails: "",
		OOB:           true,
	}
	return p.tmpl.ExecuteTemplate(w, "slideover_container", view)
}

func (p *Presentation) RenderSlideoverWithDetails(w io.Writer, detailsView any) error {
	var buf bytes.Buffer

	switch v := detailsView.(type) {
	case CategoryView:
		if err := p.tmpl.ExecuteTemplate(&buf, "category_details", v); err != nil {
			return err
		}
	case TaskView:
		if err := p.tmpl.ExecuteTemplate(&buf, "details", v); err != nil {
			return err
		}
	case SubtaskView:
		if err := p.tmpl.ExecuteTemplate(&buf, "subtask_details", v); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown details view type: %T", v)
	}

	view := PageView{
		ActiveDetails: template.HTML(buf.String()),
		OOB:           true,
	}
	return p.tmpl.ExecuteTemplate(w, "slideover_container", view)
}
