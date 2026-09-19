package service

import (
	"nysoure/server/ctx"
	"nysoure/server/dao"
	"nysoure/server/model"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxTagTemplateNameLength    = 64
	maxTagTemplateContentLength = 2048
	maxTagTemplatesPerUser      = 100
)

func extractTemplateParams(content string) ([]string, error) {
	params := make([]string, 0)
	seen := make(map[string]struct{})
	runes := []rune(content)
	for i := 0; i < len(runes); {
		if runes[i] == '}' {
			return nil, model.NewRequestError("invalid template: unmatched '}'")
		}
		if runes[i] != '{' {
			i++
			continue
		}
		i++
		start := i
		for i < len(runes) && runes[i] != '}' {
			if runes[i] == '{' {
				return nil, model.NewRequestError("invalid template: nested '{'")
			}
			i++
		}
		if i >= len(runes) {
			return nil, model.NewRequestError("invalid template: unmatched '{'")
		}
		name := string(runes[start:i])
		if err := validateTemplateParamName(name); err != nil {
			return nil, err
		}
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			params = append(params, name)
		}
		i++
	}
	return params, nil
}

func validateTemplateParamName(name string) error {
	if name == "" {
		return model.NewRequestError("invalid template: empty parameter name")
	}
	for _, r := range name {
		if r == ',' || unicode.IsSpace(r) {
			return model.NewRequestError("invalid template: invalid parameter name")
		}
	}
	return nil
}

func templateHasTagParts(content string) bool {
	for _, part := range strings.Split(content, ",") {
		if strings.TrimSpace(part) != "" {
			return true
		}
	}
	return false
}

func ValidateTagTemplate(name, content string) ([]string, error) {
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	if name == "" {
		return nil, model.NewRequestError("template name is required")
	}
	if utf8.RuneCountInString(name) > maxTagTemplateNameLength {
		return nil, model.NewRequestError("template name too long")
	}
	if content == "" {
		return nil, model.NewRequestError("template content is required")
	}
	if utf8.RuneCountInString(content) > maxTagTemplateContentLength {
		return nil, model.NewRequestError("template content too long")
	}
	params, err := extractTemplateParams(content)
	if err != nil {
		return nil, err
	}
	if !templateHasTagParts(content) {
		return nil, model.NewRequestError("template produces no tags")
	}
	return params, nil
}

func ExpandTagTemplate(content string, values map[string]string) ([]string, error) {
	if _, err := extractTemplateParams(content); err != nil {
		return nil, err
	}
	if values == nil {
		values = map[string]string{}
	}
	var b strings.Builder
	runes := []rune(content)
	for i := 0; i < len(runes); {
		if runes[i] != '{' {
			b.WriteRune(runes[i])
			i++
			continue
		}
		i++
		start := i
		for i < len(runes) && runes[i] != '}' {
			i++
		}
		name := string(runes[start:i])
		raw, ok := values[name]
		if !ok {
			return nil, model.NewRequestError("missing parameter: " + name)
		}
		v := strings.TrimSpace(raw)
		if v == "" {
			return nil, model.NewRequestError("parameter cannot be empty: " + name)
		}
		if strings.ContainsAny(v, "{},") {
			return nil, model.NewRequestError("parameter value cannot contain braces or commas: " + name)
		}
		b.WriteString(v)
		i++
	}
	parts := strings.Split(b.String(), ",")
	tags := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		tagName := strings.TrimSpace(part)
		if tagName == "" {
			continue
		}
		if utf8.RuneCountInString(tagName) > maxTagLength {
			return nil, model.NewRequestError("Tag name too long: " + tagName)
		}
		if strings.Contains(tagName, "%") {
			return nil, model.NewRequestError("Tag name cannot contain '%' character")
		}
		if _, ok := seen[tagName]; ok {
			continue
		}
		seen[tagName] = struct{}{}
		tags = append(tags, tagName)
	}
	if len(tags) == 0 {
		return nil, model.NewRequestError("template produces no tags")
	}
	return tags, nil
}

func tagTemplateToView(t model.TagTemplate) model.TagTemplateView {
	params, err := extractTemplateParams(t.Content)
	if err != nil || params == nil {
		params = []string{}
	}
	return model.TagTemplateView{
		ID:        t.ID,
		Name:      t.Name,
		Content:   t.Content,
		Params:    params,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func requireTemplateUser(c ctx.Context) (uint, error) {
	uid, ok := c.UserID()
	if !ok {
		return 0, model.NewUnAuthorizedError("Unauthorized")
	}
	if c.UserPermission() < model.PermissionUploader {
		return 0, model.NewUnAuthorizedError("User cannot manage tag templates")
	}
	return uid, nil
}

func getOwnedTagTemplate(uid, id uint) (model.TagTemplate, error) {
	t, err := dao.GetTagTemplateByID(id)
	if err != nil {
		return model.TagTemplate{}, err
	}
	if t.UserID != uid {
		return model.TagTemplate{}, model.NewNotFoundError("Tag template not found")
	}
	return t, nil
}

func CreateTagTemplate(c ctx.Context, name, content string) (*model.TagTemplateView, error) {
	uid, err := requireTemplateUser(c)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	if _, err := ValidateTagTemplate(name, content); err != nil {
		return nil, err
	}
	count, err := dao.CountTagTemplatesByUser(uid)
	if err != nil {
		return nil, err
	}
	if count >= maxTagTemplatesPerUser {
		return nil, model.NewRequestError("too many tag templates")
	}
	if _, err := dao.GetTagTemplateByUserAndName(uid, name); err == nil {
		return nil, model.NewRequestError("template name already exists")
	} else if !model.IsNotFoundError(err) {
		return nil, err
	}
	t, err := dao.CreateTagTemplate(uid, name, content)
	if err != nil {
		return nil, err
	}
	view := tagTemplateToView(t)
	return &view, nil
}

func UpdateTagTemplate(c ctx.Context, id uint, name, content string) (*model.TagTemplateView, error) {
	uid, err := requireTemplateUser(c)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	if _, err := ValidateTagTemplate(name, content); err != nil {
		return nil, err
	}
	if _, err := getOwnedTagTemplate(uid, id); err != nil {
		return nil, err
	}
	existing, err := dao.GetTagTemplateByUserAndName(uid, name)
	if err == nil && existing.ID != id {
		return nil, model.NewRequestError("template name already exists")
	} else if err != nil && !model.IsNotFoundError(err) {
		return nil, err
	}
	if err := dao.UpdateTagTemplate(id, name, content); err != nil {
		return nil, err
	}
	t, err := dao.GetTagTemplateByID(id)
	if err != nil {
		return nil, err
	}
	view := tagTemplateToView(t)
	return &view, nil
}

func DeleteTagTemplate(c ctx.Context, id uint) error {
	uid, err := requireTemplateUser(c)
	if err != nil {
		return err
	}
	if _, err := getOwnedTagTemplate(uid, id); err != nil {
		return err
	}
	return dao.DeleteTagTemplate(id)
}

func ListTagTemplates(c ctx.Context) ([]model.TagTemplateView, error) {
	uid, err := requireTemplateUser(c)
	if err != nil {
		return nil, err
	}
	templates, err := dao.ListTagTemplatesByUser(uid)
	if err != nil {
		return nil, err
	}
	views := make([]model.TagTemplateView, 0, len(templates))
	for _, t := range templates {
		views = append(views, tagTemplateToView(t))
	}
	return views, nil
}

func ApplyTagTemplate(c ctx.Context, id uint, params map[string]string) ([]model.TagView, error) {
	uid, err := requireTemplateUser(c)
	if err != nil {
		return nil, err
	}
	t, err := getOwnedTagTemplate(uid, id)
	if err != nil {
		return nil, err
	}
	names, err := ExpandTagTemplate(t.Content, params)
	if err != nil {
		return nil, err
	}
	return GetOrCreateTags(c, names, "")
}
