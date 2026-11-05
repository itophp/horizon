package articles

import ar "baseline/services/api/kernel/applicationruntime"

// Application contributes presentation only; all data and writes retain their existing contracts.
func Application() ar.Application {
	pages := []ar.Page{}
	for _, kind := range []string{"list", "detail", "form"} {
		names := []string{"title", "status", "revision", "reviewCode"}
		if kind == "detail" {
			names = append(names, "body")
		}
		if kind == "form" {
			names = []string{"title", "body"}
		}
		p := ar.Page{SchemaVersion: ar.Version, ID: "article." + kind, Type: kind, TitleKey: "articles:heading", EntityRef: "article", Layout: "standard", Fields: []ar.Field{}, ReadCapability: "article.get"}
		if kind == "list" {
			p.ReadCapability = "article.list"
		}
		if kind == "form" {
			p.CreateCapability = "article.create"
			p.UpdateCapability = "article.update"
		}
		for _, name := range names {
			key := "articles:" + name
			component := "text"
			if name == "revision" {
				key = "articles:article.revision"
			}
			if name == "reviewCode" {
				key = "articles:reason"
			}
			if name == "status" {
				component = "status"
			}
			if name == "body" {
				component = "textarea"
			}
			prefix := ""
			if name == "status" {
				prefix = "articles:states."
			}
			if name == "reviewCode" {
				prefix = "articles:codes."
			}
			p.Fields = append(p.Fields, ar.Field{Name: name, LabelKey: key, Component: component, Link: name == "title", ValueLabelPrefix: prefix})
		}
		read := definition("get")
		if kind == "list" {
			read = definition("list")
		}
		adapted, err := ar.Adapt(p, articleEntity(), read, definition("update"))
		if err != nil {
			panic(err)
		}
		pages = append(pages, adapted)
	}
	return ar.Stamp(ar.Application{ID: "articles", Version: "1.0.0", LabelKey: "articles:heading", Route: "/articles", Pages: pages})
}
