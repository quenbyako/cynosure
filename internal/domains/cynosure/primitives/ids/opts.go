package ids

func WithSlug(slug string) slugOption { return slugOption{slug: slug} }

type slugOption struct{ slug string }

func (s slugOption) applyToolID(t *ToolID)       { t.slug = s.slug }
func (s slugOption) applyAccountID(a *AccountID) { a.slug = s.slug }
