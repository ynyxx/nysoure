package service

// https://api.vndb.org/kana

import (
	"errors"
	"fmt"
	"math"
	"nysoure/server/ai"
	"nysoure/server/cache"
	"nysoure/server/ctx"
	"nysoure/server/dao"
	"nysoure/server/model"
	"strconv"
	"strings"
	"time"

	govndb "git.nyne.dev/o/go_vndb"
	"github.com/gofiber/fiber/v3/log"
)

const (
	tagTypes        = "剧情,游戏类型,人物特点,场景,其他"
	tagTypeVA       = "声优"
	tagTypeProducer = "会社"
	tagTypePrompt   = `你是一个视觉小说标签分类助手。

请判断下面这个标签最适合归入哪个类型。
可选类型只有这几个: %s。

要求:
1. 只能输出一个类型名。
2. 输出必须严格等于可选类型中的一个值。
3. 如果无法确定, 输出"其他"。

标签: %s`
	vndbTagTypeContent = "cont"
)

// GetInfoFromVndb returns character information and release date for a given VNDB ID.
func GetInfoFromVndb(vnID string, c ctx.Context) ([]CharacterParams, string, error) {
	if c.UserPermission() < model.PermissionUploader {
		return nil, "", model.NewUnAuthorizedError("You have not permission to fetch characters from VNDB")
	}

	result, err := govndb.GetVN(vnID)
	if err != nil {
		return nil, "", model.NewInternalServerError("Failed to fetch data from VNDB")
	}
	characters, err := charactersFromVndb(result)
	if err != nil {
		return nil, "", model.NewInternalServerError("Failed to process character data from VNDB")
	}

	released := ""
	if result.Released != nil {
		released = *result.Released
	}

	return characters, released, nil
}

func charactersFromVndb(vn *govndb.VN) ([]CharacterParams, error) {
	var characters []CharacterParams
	processedCharacters := make(map[string]bool) // 避免重复角色

	// 遍历声优信息
	for _, va := range vn.VoiceActors {
		if va.Character == nil {
			continue
		}

		role := "Unknown"
		for _, vnc := range va.Character.VNs {
			if vnc.ID == vn.ID && vnc.Role != nil {
				role = *vnc.Role
				break
			}
		}

		if role != "primary" && role != "side" && role != "main" {
			continue
		}

		// 避免重复角色
		if processedCharacters[va.Character.ID] {
			continue
		}
		processedCharacters[va.Character.ID] = true

		characterName := strings.ReplaceAll(va.Character.OriginalName(), " ", "")
		if characterName == "" {
			continue // 跳过没有名字的角色
		}

		// 使用 original 字段作为声优名，如果没有则使用 name
		cvName := ""
		if va.Staff != nil {
			cvName = strings.ReplaceAll(va.Staff.OriginalName(), " ", "")
		}
		if cvName == "" && va.Staff != nil {
			cvName = va.Staff.Name
		}

		character := CharacterParams{
			Name:  characterName,
			Alias: []string{},
			CV:    cvName,
			Role:  role,
			Image: 0, // 默认值，下面会下载图片
		}

		// 下载并保存角色图片
		if va.Character.Image != nil && va.Character.Image.URL != "" {
			imageID, err := downloadAndCreateImage(va.Character.Image.URL)
			if err != nil {
				log.Error("Failed to download character image:", err)
				// 继续处理，即使图片下载失败
			} else {
				character.Image = imageID
			}
		}

		characters = append(characters, character)
	}
	return characters, nil
}

func getVNDBRating(vnID string) (int, error) {
	rating, err := govndb.GetVNRating(vnID)
	if err != nil {
		return 0, model.NewInternalServerError("Failed to get VNDB rating")
	}
	intRating := 0
	if rating != nil {
		intRating = int(math.Round(float64(*rating)))
	}
	return intRating, nil
}

func getVNDBRatingWithCache(vnID string) (int, error) {
	cacheKey := fmt.Sprintf("vndb_rating_%s", vnID)
	ratingStr, err := cache.Get(cacheKey)
	if err != nil && !errors.Is(err, cache.ErrNotFound) {
		return 0, err
	} else if errors.Is(err, cache.ErrNotFound) {
		rating, err := getVNDBRating(vnID)
		if err != nil {
			return 0, err
		}
		err = cache.Set(cacheKey, strconv.Itoa(rating), 24*time.Hour)
		if err != nil {
			log.Error("Failed to set VNDB rating cache: ", err)
		}
		return rating, nil
	}
	rating, err := strconv.Atoi(ratingStr)
	if err != nil {
		return 0, model.NewInternalServerError("Failed to parse VNDB rating")
	}
	return rating, nil
}

type ResourceFormPrefill struct {
	Title             string            `json:"title"`
	AlternativeTitles []string          `json:"alternative_titles"`
	Links             []model.Link      `json:"links"`
	ReleaseDate       string            `json:"release_date"`
	Tags              []model.TagView   `json:"tags"`
	Article           string            `json:"article"`
	Images            []uint            `json:"images"`
	CoverID           *uint             `json:"cover_id"`
	Gallery           []uint            `json:"gallery"`
	GalleryNsfw       []uint            `json:"gallery_nsfw"`
	Characters        []CharacterParams `json:"characters"`
}

// PrefillSections controls which sections are fetched from VNDB.
type PrefillSections struct {
	Basic      bool // title, alternative_titles, links, release_date
	Article    bool // article
	Tags       bool // tags
	Images     bool // images, cover_id
	Characters bool // characters
}

// ParsePrefillSections parses a comma-separated sections string.
// An empty string means all sections are included.
func ParsePrefillSections(raw string) PrefillSections {
	if raw == "" {
		return PrefillSections{Basic: true, Article: true, Tags: true, Images: true, Characters: true}
	}
	s := PrefillSections{}
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(part) {
		case "basic":
			s.Basic = true
		case "article":
			s.Article = true
		case "tags":
			s.Tags = true
		case "images":
			s.Images = true
		case "characters":
			s.Characters = true
		}
	}
	return s
}

func GetResourceFormPrefillFromVNDB(vnID string, c ctx.Context, sections PrefillSections) (*ResourceFormPrefill, error) {
	if c.UserPermission() < model.PermissionUploader {
		return nil, model.NewUnAuthorizedError("You have not permission to fetch resource params from VNDB")
	}

	params, err := ResourceParamsFromVNDB(vnID, sections)
	if err != nil {
		return nil, err
	}

	tags := make([]model.TagView, 0, len(params.Tags))
	for _, tagID := range params.Tags {
		tag, err := GetTag(tagID)
		if err != nil {
			return nil, err
		}
		tags = append(tags, *tag)
	}

	return &ResourceFormPrefill{
		Title:             params.Title,
		AlternativeTitles: params.AlternativeTitles,
		Links:             params.Links,
		ReleaseDate:       params.ReleaseDate,
		Tags:              tags,
		Article:           params.Article,
		Images:            params.Images,
		CoverID:           params.CoverID,
		Gallery:           params.Gallery,
		GalleryNsfw:       params.GalleryNsfw,
		Characters:        params.Characters,
	}, nil
}

func ResourceParamsFromVNDB(vnid string, sections PrefillSections) (*ResourceParams, error) {
	vn, err := govndb.GetVN(vnid)
	if err != nil {
		return nil, model.NewRequestError(fmt.Sprintf("Error fetching vndb: %s", err.Error()))
	}
	if vn.Released == nil {
		vn.Released = new(string)
	}
	if vn.Description == nil {
		vn.Description = new(string)
	}
	params := &ResourceParams{
		Title: vn.OriginalTitle(),
		Links: []model.Link{
			{
				URL:   fmt.Sprintf("https://vndb.org/%s", vnid),
				Label: "VNDB",
			},
		},
		ReleaseDate: *vn.Released,
	}

	if sections.Article || sections.Tags {
		type translationReq struct {
			Description string `json:"description"`
			Tags        []struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			} `json:"tags"`
		}

		data := translationReq{
			Description: *vn.Description,
		}
		for _, tag := range vn.Tags {
			rating := 0.0
			if tag.Rating != nil {
				rating = *tag.Rating
			}
			if rating < 2.0 {
				continue
			}
			if tag.Category == vndbTagTypeContent {
				data.Tags = append(data.Tags, struct {
					Name string `json:"name"`
					ID   string `json:"id"`
				}{
					Name: tag.Name,
					ID:   tag.ID,
				})
			}
		}
		characterNames := make([]string, 0, len(vn.VoiceActors))
		for _, va := range vn.VoiceActors {
			characterNames = append(characterNames, va.Character.OriginalName())
		}
		aiCtx := "你需要翻译的是一个视觉小说的简介和标签。简介可能包含一些专业术语, 标签可能包含一些专有名词。请将简介和标签翻译为流畅的中文, 并保持标签的ID原样不变。"
		if len(characterNames) > 0 {
			aiCtx += fmt.Sprintf("这个视觉小说包含以下角色: %s。", strings.Join(characterNames, ","))
		}
		data, err = ai.Translate(data, aiCtx)
		if err != nil {
			return nil, model.NewInternalServerError("Failed to translate VNDB content")
		}

		if sections.Article {
			params.Article = data.Description
		}

		if sections.Tags {
			tagIDs := []uint{}
			for _, tag := range data.Tags {
				tagID, err := tagIDFromVNDB(tag.Name, tag.ID)
				if err != nil {
					return nil, err
				}
				tagIDs = append(tagIDs, tagID)
			}

			for _, va := range vn.VoiceActors {
				vaName := va.Staff.OriginalName()
				vaid := va.Staff.ID
				tagID, err := tagIDFromVA(vaName, vaid)
				if err != nil {
					log.Error("Failed to get tag ID from VA: ", err)
					continue
				}
				tagIDs = append(tagIDs, tagID)
			}

			for _, producer := range vn.Developers {
				producerName := producer.Name
				if producer.Original != nil && *producer.Original != "" {
					producerName = *producer.Original
				}
				tagID, err := tagIDFromProducer(producerName)
				if err != nil {
					log.Error("Failed to get tag ID from producer: ", err)
					continue
				}
				tagIDs = append(tagIDs, tagID)
			}

			for _, staff := range vn.Staff {
				staffName := staff.OriginalName()
				tagID, err := tagIDFromStaffIfExists(staffName)
				if err != nil {
					log.Error("Failed to get tag ID from staff: ", err)
					continue
				}
				if tagID != 0 {
					tagIDs = append(tagIDs, tagID)
				}
			}

			// 标签去重
			tagIDSet := make(map[uint]struct{})
			uniqueTagIDs := []uint{}
			for _, id := range tagIDs {
				if _, exists := tagIDSet[id]; !exists {
					tagIDSet[id] = struct{}{}
					uniqueTagIDs = append(uniqueTagIDs, id)
				}
			}
			params.Tags = uniqueTagIDs
		}
	}

	if sections.Images {
		// 封面
		if vn.Image.URL != "" {
			imageID, err := downloadAndCreateImage(vn.Image.URL)
			if err != nil {
				log.Error("Failed to download VN cover image:", err)
			} else {
				params.Images = []uint{imageID}
				params.CoverID = &imageID
			}
		}
		if params.CoverID != nil && sections.Article {
			params.Article = fmt.Sprintf("![image](/image/%d)\n\n%s", *params.CoverID, params.Article)
		}
	}

	if sections.Characters {
		characters, err := charactersFromVndb(vn)
		if err != nil {
			log.Error("Failed to process character data from VNDB: ", err)
		}
		params.Characters = characters
	}

	return params, nil
}

func tagIDFromVNDB(name string, vnid string) (uint, error) {
	t, err := dao.GetTagByVNID(vnid)
	if err != nil && !model.IsNotFoundError(err) {
		return 0, err
	} else if model.IsNotFoundError(err) {
		t, err = dao.GetTagByName(name)
		if err != nil && !model.IsNotFoundError(err) {
			return 0, err
		} else if model.IsNotFoundError(err) {
			tagType, err := decideTagType(name)
			if err != nil {
				return 0, err
			}
			t, err = dao.CreateTagWithVNID(name, tagType, vnid)
			if err != nil {
				return 0, err
			}
		}
	}
	return t.ID, nil
}

func decideTagType(name string) (string, error) {
	allowedTypes := strings.Split(tagTypes, ",")
	prompt := fmt.Sprintf(tagTypePrompt, tagTypes, name)
	response := strings.TrimSpace(ai.Chat(prompt))
	if response == "" {
		return "", model.NewInternalServerError("Failed to classify tag type")
	}

	response = strings.Trim(response, "`\"'\n\r\t ")
	for _, tagType := range allowedTypes {
		if response == tagType {
			return tagType, nil
		}
	}

	for _, tagType := range allowedTypes {
		if strings.Contains(response, tagType) {
			return tagType, nil
		}
	}

	return "", model.NewInternalServerError(fmt.Sprintf("Invalid tag type returned by AI: %s", response))
}

func tagIDFromVA(name string, vnid string) (uint, error) {
	name = strings.ReplaceAll(name, " ", "")
	t, err := dao.GetTagByName(name)
	if err != nil && !model.IsNotFoundError(err) {
		return 0, err
	} else if model.IsNotFoundError(err) {
		// 可能是别名, 查询声优完整信息
		staff, err := govndb.GetStaff(vnid)
		if err != nil {
			return 0, err
		}
		realName := strings.ReplaceAll(staff.OriginalName(), " ", "")
		realNameTag, err := dao.GetTagByName(realName)
		if err != nil && !model.IsNotFoundError(err) {
			return 0, err
		} else if model.IsNotFoundError(err) {
			// 仍然没有找到, 创建一个新的声优标签
			// 先创建真名标签
			realNameTag, err = dao.CreateTagWithVNID(realName, tagTypeVA, vnid)
			if err != nil {
				return 0, err
			}
			if name != realName {
				// 创建别名标签
				t, err = dao.CreateTagWithVNID(name, tagTypeVA, vnid)
				if err != nil {
					return 0, err
				}
				err = dao.SetTagAlias(realNameTag.ID, name)
				if err != nil {
					return 0, err
				}
				return t.ID, nil
			} else {
				return realNameTag.ID, nil
			}
		} else {
			// 找到了真名标签, 创建一个别名标签
			t, err = dao.CreateTagWithVNID(name, tagTypeVA, vnid)
			if err != nil {
				return 0, err
			}
			err = dao.SetTagAlias(realNameTag.ID, name)
			if err != nil {
				return 0, err
			}
			return t.ID, nil
		}
	}
	return t.ID, nil
}

func tagIDFromProducer(name string) (uint, error) {
	t, err := dao.GetTagByName(name)
	if err != nil && !model.IsNotFoundError(err) {
		return 0, err
	} else if model.IsNotFoundError(err) {
		t, err = dao.CreateTagWithType(name, tagTypeProducer)
		if err != nil {
			return 0, err
		}
	}
	return t.ID, nil
}

func tagIDFromStaffIfExists(name string) (uint, error) {
	// 对于staff标签, 直接查询名字, 不创建新标签
	t, err := dao.GetTagByName(name)
	if err != nil {
		if model.IsNotFoundError(err) {
			// 去除空格后再试一次
			nameNoSpace := strings.ReplaceAll(name, " ", "")
			t, err = dao.GetTagByName(nameNoSpace)
			if err != nil {
				if model.IsNotFoundError(err) {
					return 0, nil // 不存在则返回0, 不创建新标签
				}
				return 0, err
			}
			return t.ID, nil
		}
		return 0, err
	}
	return t.ID, nil
}
