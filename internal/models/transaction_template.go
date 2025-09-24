package models

import (
	"math/big"
	"sort"
	"strings"
	"time"
)

type TransactionTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	ToAddress   string    `json:"to_address"`
	ContactName string    `json:"contact_name,omitempty"`
	Amount      *big.Int  `json:"amount,omitempty"`
	Asset       string    `json:"asset"`
	Notes       string    `json:"notes,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UseCount    int       `json:"use_count"`
	IsFavorite  bool      `json:"is_favorite"`
}

type TransactionTemplateManager struct {
	templates []TransactionTemplate `json:"templates"`
}

func NewTransactionTemplateManager() *TransactionTemplateManager {
	return &TransactionTemplateManager{
		templates: make([]TransactionTemplate, 0),
	}
}

func NewTransactionTemplate(name, description, toAddress, contactName, asset, notes string, amount *big.Int, tags []string) *TransactionTemplate {
	return &TransactionTemplate{
		ID:          generateTemplateID(),
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		ToAddress:   strings.TrimSpace(toAddress),
		ContactName: strings.TrimSpace(contactName),
		Amount:      amount,
		Asset:       asset,
		Notes:       strings.TrimSpace(notes),
		Tags:        tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		UseCount:    0,
		IsFavorite:  false,
	}
}

func (tt *TransactionTemplate) Update(name, description, toAddress, contactName, asset, notes string, amount *big.Int, tags []string) {
	if name != "" {
		tt.Name = strings.TrimSpace(name)
	}
	if description != "" {
		tt.Description = strings.TrimSpace(description)
	}
	if toAddress != "" {
		tt.ToAddress = strings.TrimSpace(toAddress)
	}
	if contactName != "" {
		tt.ContactName = strings.TrimSpace(contactName)
	}
	if asset != "" {
		tt.Asset = asset
	}
	if amount != nil {
		tt.Amount = amount
	}
	tt.Notes = strings.TrimSpace(notes)
	tt.Tags = tags
	tt.UpdatedAt = time.Now()
}

func (tt *TransactionTemplate) Use() {
	tt.UseCount++
	tt.UpdatedAt = time.Now()
}

func (tt *TransactionTemplate) ToggleFavorite() {
	tt.IsFavorite = !tt.IsFavorite
	tt.UpdatedAt = time.Now()
}

func (ttm *TransactionTemplateManager) AddTemplate(template *TransactionTemplate) {
	ttm.templates = append(ttm.templates, *template)
	ttm.sortTemplates()
}

func (ttm *TransactionTemplateManager) RemoveTemplate(id string) bool {
	for i, template := range ttm.templates {
		if template.ID == id {
			ttm.templates = append(ttm.templates[:i], ttm.templates[i+1:]...)
			return true
		}
	}
	return false
}

func (ttm *TransactionTemplateManager) FindByID(id string) *TransactionTemplate {
	for i, template := range ttm.templates {
		if template.ID == id {
			return &ttm.templates[i]
		}
	}
	return nil
}

func (ttm *TransactionTemplateManager) FindByName(name string) *TransactionTemplate {
	for i, template := range ttm.templates {
		if strings.EqualFold(template.Name, name) {
			return &ttm.templates[i]
		}
	}
	return nil
}

func (ttm *TransactionTemplateManager) GetAllTemplates() []TransactionTemplate {
	result := make([]TransactionTemplate, len(ttm.templates))
	copy(result, ttm.templates)
	return result
}

func (ttm *TransactionTemplateManager) GetFavoriteTemplates() []TransactionTemplate {
	var favorites []TransactionTemplate
	for _, template := range ttm.templates {
		if template.IsFavorite {
			favorites = append(favorites, template)
		}
	}

	// Sort favorites by use count and recency
	sort.Slice(favorites, func(i, j int) bool {
		if favorites[i].UseCount != favorites[j].UseCount {
			return favorites[i].UseCount > favorites[j].UseCount
		}
		return favorites[i].UpdatedAt.After(favorites[j].UpdatedAt)
	})

	return favorites
}

func (ttm *TransactionTemplateManager) GetRecentTemplates(limit int) []TransactionTemplate {
	if limit <= 0 {
		limit = 10
	}

	// Sort by last updated
	sorted := make([]TransactionTemplate, len(ttm.templates))
	copy(sorted, ttm.templates)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	return sorted
}

func (ttm *TransactionTemplateManager) GetMostUsedTemplates(limit int) []TransactionTemplate {
	if limit <= 0 {
		limit = 10
	}

	// Sort by use count
	sorted := make([]TransactionTemplate, len(ttm.templates))
	copy(sorted, ttm.templates)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].UseCount != sorted[j].UseCount {
			return sorted[i].UseCount > sorted[j].UseCount
		}
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	return sorted
}

func (ttm *TransactionTemplateManager) SearchTemplates(query string) []TransactionTemplate {
	if query == "" {
		return ttm.GetAllTemplates()
	}

	query = strings.ToLower(strings.TrimSpace(query))
	var results []TransactionTemplate

	for _, template := range ttm.templates {
		// Search in name, description, contact name, notes, and tags
		if strings.Contains(strings.ToLower(template.Name), query) ||
			strings.Contains(strings.ToLower(template.Description), query) ||
			strings.Contains(strings.ToLower(template.ContactName), query) ||
			strings.Contains(strings.ToLower(template.Notes), query) ||
			strings.Contains(strings.ToLower(template.ToAddress), query) ||
			containsTag(template.Tags, query) {
			results = append(results, template)
		}
	}

	// Sort results by relevance (favorites first, then by use count)
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsFavorite != results[j].IsFavorite {
			return results[i].IsFavorite
		}
		if results[i].UseCount != results[j].UseCount {
			return results[i].UseCount > results[j].UseCount
		}
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})

	return results
}

func (ttm *TransactionTemplateManager) GetTemplatesByTag(tag string) []TransactionTemplate {
	var results []TransactionTemplate
	tag = strings.ToLower(strings.TrimSpace(tag))

	for _, template := range ttm.templates {
		if containsTag(template.Tags, tag) {
			results = append(results, template)
		}
	}

	return results
}

func (ttm *TransactionTemplateManager) GetAllTags() []string {
	tagMap := make(map[string]bool)

	for _, template := range ttm.templates {
		for _, tag := range template.Tags {
			tagMap[strings.ToLower(tag)] = true
		}
	}

	var tags []string
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	sort.Strings(tags)
	return tags
}

func (ttm *TransactionTemplateManager) GetStats() (int, int, time.Time) {
	totalTemplates := len(ttm.templates)
	favoriteCount := 0
	var lastUsed time.Time

	for _, template := range ttm.templates {
		if template.IsFavorite {
			favoriteCount++
		}
		if template.UpdatedAt.After(lastUsed) {
			lastUsed = template.UpdatedAt
		}
	}

	return totalTemplates, favoriteCount, lastUsed
}

func (ttm *TransactionTemplateManager) sortTemplates() {
	// Sort by favorites first, then by use count, then by update time
	sort.Slice(ttm.templates, func(i, j int) bool {
		if ttm.templates[i].IsFavorite != ttm.templates[j].IsFavorite {
			return ttm.templates[i].IsFavorite
		}
		if ttm.templates[i].UseCount != ttm.templates[j].UseCount {
			return ttm.templates[i].UseCount > ttm.templates[j].UseCount
		}
		return ttm.templates[i].UpdatedAt.After(ttm.templates[j].UpdatedAt)
	})
}

// Maintenance methods
func (ttm *TransactionTemplateManager) CleanupUnusedTemplates(maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}

	cutoff := time.Now().Add(-maxAge)
	originalCount := len(ttm.templates)

	// Filter out old unused templates (keep favorites and used templates)
	filtered := make([]TransactionTemplate, 0, len(ttm.templates))
	for _, template := range ttm.templates {
		if template.IsFavorite || template.UseCount > 0 || template.UpdatedAt.After(cutoff) {
			filtered = append(filtered, template)
		}
	}

	ttm.templates = filtered
	return originalCount - len(ttm.templates)
}

// Export/Import methods for backup/restore
func (ttm *TransactionTemplateManager) Export() []TransactionTemplate {
	result := make([]TransactionTemplate, len(ttm.templates))
	copy(result, ttm.templates)
	return result
}

func (ttm *TransactionTemplateManager) Import(templates []TransactionTemplate) {
	ttm.templates = make([]TransactionTemplate, 0, len(templates))

	for _, template := range templates {
		ttm.templates = append(ttm.templates, template)
	}

	ttm.sortTemplates()
}

// Helper functions
func generateTemplateID() string {
	return "template_" + time.Now().Format("20060102150405")
}

func containsTag(tags []string, query string) bool {
	query = strings.ToLower(query)
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
