package collector

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/wenkaler/xfreehack/model"
)

const (
	BaseURL = "https://lovikod.ru"
)

type Storage interface {
	SaveCategory(c model.Category) (int, error)
	SaveStore(s model.Store) (int, error)
	SaveCoupon(c model.Coupon) error
}

type Config struct {
	Logger  log.Logger
	Storage Storage
}

type Collector struct {
	cfg *Config
}

func New(cfg *Config) *Collector {
	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}
	return &Collector{
		cfg: cfg,
	}
}

func (c *Collector) CollectFor(url string) error {
	// Legacy support or specific trigger
	return c.CollectAll()
}

func (c *Collector) CollectAll() error {
	level.Info(c.cfg.Logger).Log("msg", "starting collection from lovikod.ru")

	// 1. Get Categories
	categories, err := c.parseCategories()
	if err != nil {
		return fmt.Errorf("failed to parse categories: %w", err)
	}

	for _, cat := range categories {
		level.Info(c.cfg.Logger).Log("msg", "processing category", "name", cat.Name)
		catID, err := c.cfg.Storage.SaveCategory(cat)
		if err != nil {
			level.Error(c.cfg.Logger).Log("msg", "failed to save category", "name", cat.Name, "err", err)
			continue
		}
		cat.ID = catID

		// 2. Get Stores for Category
		stores, err := c.parseStores(cat)
		if err != nil {
			level.Error(c.cfg.Logger).Log("msg", "failed to parse stores", "category", cat.Name, "err", err)
			continue
		}

		for _, store := range stores {
			store.CategoryID = cat.ID
			level.Debug(c.cfg.Logger).Log("msg", "processing store", "name", store.Name)
			storeID, err := c.cfg.Storage.SaveStore(store)
			if err != nil {
				level.Error(c.cfg.Logger).Log("msg", "failed to save store", "name", store.Name, "err", err)
				continue
			}
			store.ID = storeID

			// 3. Get Coupons for Store
			coupons, err := c.parseCoupons(store)
			if err != nil {
				level.Error(c.cfg.Logger).Log("msg", "failed to parse coupons", "store", store.Name, "err", err)
				continue
			}

			for _, coupon := range coupons {
				coupon.StoreID = store.ID
				if err := c.cfg.Storage.SaveCoupon(coupon); err != nil {
					level.Error(c.cfg.Logger).Log("msg", "failed to save coupon", "code", coupon.Code, "err", err)
				}
			}

			// Be polite
			time.Sleep(500 * time.Millisecond)
		}
	}
	level.Info(c.cfg.Logger).Log("msg", "collection finished")
	return nil
}

func (c *Collector) parseCategories() ([]model.Category, error) {
	// Start at home page to find the main nav
	doc, err := c.fetch(BaseURL + "/")
	if err != nil {
		return nil, err
	}

	var categories []model.Category
	// Updated selector based on browser analysis: Look for links in the primary nav
	doc.Find("ul.uk-nav.uk-nav-primary a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || strings.HasPrefix(href, "http") || href == "/" || href == "#" {
			return
		}
		// Basic filtering to avoid junk links
		if len(href) < 2 {
			return
		}

		name := strings.TrimSpace(s.Text())
		if name == "" {
			return
		}

		slug := strings.TrimPrefix(href, "/")

		level.Info(c.cfg.Logger).Log("msg", "found category candidate", "name", name, "href", href)

		categories = append(categories, model.Category{
			Name: name,
			Slug: slug,
		})
	})

	// If generic selector fails, try fallback or specific known classes
	if len(categories) == 0 {
		level.Warn(c.cfg.Logger).Log("msg", "no categories found with primary selector, trying fallback")
	}

	// Remove duplicates
	unique := make(map[string]model.Category)
	for _, cat := range categories {
		unique[cat.Slug] = cat
	}

	res := make([]model.Category, 0, len(unique))
	for _, cat := range unique {
		res = append(res, cat)
	}

	level.Info(c.cfg.Logger).Log("msg", "parsed categories", "count", len(res))
	return res, nil
}

func (c *Collector) parseStores(cat model.Category) ([]model.Store, error) {
	url := BaseURL + "/" + cat.Slug
	doc, err := c.fetch(url)
	if err != nil {
		return nil, err
	}

	var stores []model.Store
	// Updated selector based on browser analysis: .mwall-photo-link
	doc.Find(".mwall-photo-link").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Check if it belongs to this category path to be safe
		// Example href: /knigi/promokody-litres
		if !strings.HasPrefix(href, "/"+cat.Slug+"/") {
			return
		}

		slug := strings.TrimPrefix(href, "/"+cat.Slug+"/")
		name := strings.TrimSpace(s.Text())

		// Fallback for name if text is empty (often the case for image links)
		if name == "" {
			if imgAlt, ok := s.Find("img").Attr("alt"); ok {
				name = imgAlt
			} else {
				name = slug
			}
		}

		if name == "" {
			name = "Unknown Store"
		}

		level.Debug(c.cfg.Logger).Log("msg", "found store candidate", "name", name, "href", href)

		stores = append(stores, model.Store{
			Name: name,
			Slug: slug,
			URL:  BaseURL + href,
		})
	})

	// De-dupe
	unique := make(map[string]model.Store)
	for _, s := range stores {
		unique[s.Slug] = s
	}
	res := make([]model.Store, 0, len(unique))
	for _, s := range unique {
		res = append(res, s)
	}

	level.Info(c.cfg.Logger).Log("msg", "parsed stores", "category", cat.Name, "count", len(res))
	return res, nil
}

func (c *Collector) parseCoupons(store model.Store) ([]model.Coupon, error) {
	doc, err := c.fetch(store.URL)
	if err != nil {
		return nil, err
	}

	var coupons []model.Coupon

	// Selector for coupons: looking for table rows with promo codes
	count := 0
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		// Specific check for class .promocode which usually contains the coupon code
		codeSel := s.Find(".promocode")
		if codeSel.Length() == 0 {
			return
		}
		code := strings.TrimSpace(codeSel.Text())

		if code == "" {
			return
		}

		// Find date: usually the first cell
		dateText := strings.TrimSpace(s.Find("td").First().Text())
		expiry := parseDate(dateText)

		// Find description: usually the last cell or the one with text
		// Analysis suggests 3rd column is description
		var desc string
		tds := s.Find("td")
		if tds.Length() >= 3 {
			desc = strings.TrimSpace(tds.Eq(2).Text())
		} else {
			desc = strings.TrimSpace(s.Text())
		}

		// Clean up description if it contains the code or date
		desc = strings.ReplaceAll(desc, code, "")
		desc = strings.ReplaceAll(desc, dateText, "")
		desc = strings.TrimSpace(desc)

		link, _ := codeSel.Attr("href")
		if link == "" {
			// fallback to store url
			link = store.URL
		} else if strings.HasPrefix(link, "/") {
			link = BaseURL + link
		}

		coupons = append(coupons, model.Coupon{
			StoreID:     store.ID,
			Code:        code,
			Description: desc,
			ExpiryDate:  expiry,
			Link:        link,
		})
		count++
	})

	level.Info(c.cfg.Logger).Log("msg", "parsed coupons", "store", store.Name, "count", count)

	return coupons, nil
}

func (c *Collector) fetch(url string) (*goquery.Document, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

func parseDate(d string) int64 {
	// Format "до 31.01.26"
	d = strings.TrimPrefix(d, "до ")
	d = strings.TrimSpace(d)
	// Try parse
	t, err := time.Parse("02.01.06", d) // 2-digit year
	if err != nil {
		t, err = time.Parse("02.01.2006", d)
	}
	if err != nil {
		// Return some future date or now if fail
		return time.Now().AddDate(0, 1, 0).Unix()
	}
	return t.Unix()
}
