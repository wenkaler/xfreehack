package collector

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-kit/kit/log/level"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-kit/kit/log"
)

type Config struct {
	Logger      log.Logger
	Storage     Storage
	URL         string
	NameMarkets []string
}

type Storage interface {
	GetRecordsByName(token, name string) ([]byte, error)
	Collect(records []Record) error
}

type Collector struct {
	svc *basicService
	cfg *Config
}

type Record struct {
	PostID string
	Market string
	Link   string
	Code   string
}

func New(cfg *Config) (*Collector, error) {
	//if cfg.Storage == nil {
	//	return nil, errors.New("storage is empty")
	//}
	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}
	if cfg.URL == "" {
		return nil, errors.New("URL is empty")
	}
	collector := &Collector{
		cfg: cfg,
	}
	return collector, nil
}

func (c *Collector) GetRecord(token, brand string) ([]Record, error) {
	if token == "" {
		return nil, fmt.Errorf("token is empty")
	}
	if brand == "" {
		return nil, fmt.Errorf("name is empty")
	}

	b, err := c.cfg.Storage.GetRecordsByName(token, brand)
	if err != nil {
		return nil, fmt.Errorf("failed get record from data base: %v", err)
	}
	var resp []Record
	err = json.Unmarshal(b, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed unmarshal struct record: %v", err)
	}
	return resp, nil
}

func (c *Collector) Collect() error {
	var records []Record
	resp, err := http.Get(c.cfg.URL)
	if err != nil {
		return fmt.Errorf("failed get request url: %s, reason: %v", c.cfg.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("failed request got status code %v", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed create newDocument: %v", err)
	}
	doc.Find("article").Each(func(i int, s *goquery.Selection) {
		var r Record
		if postID, exist := s.Attr("id"); exist {
			r.PostID = postID
			s.Find("a").Each(func(i int, s *goquery.Selection) {
				for _, name := range c.cfg.NameMarkets {
					b, err := regexp.MatchString(name, s.Text())
					if err != nil {
						level.Error(c.cfg.Logger).Log("msg", "failed regexp", "err", err)
						continue
					}
					if b {
						r.Market = name
						link, exist := s.Attr("href")
						if !exist {
							level.Error(c.cfg.Logger).Log("msg", "href does not exist in <a>", "err", err)
							continue
						}
						resp, err := http.Get(link)
						if err != nil {
							level.Error(c.cfg.Logger).Log("msg", "get link failed", "link", link, "err", err)
							continue
						}
						if resp.StatusCode < 200 || resp.StatusCode > 299 {
							level.Error(c.cfg.Logger).Log("msg", "failed request", "link", link, "status code", resp.StatusCode)
							continue
						}

						doc, err := goquery.NewDocumentFromReader(resp.Body)
						if err != nil {
							resp.Body.Close()
							level.Error(c.cfg.Logger).Log("msg", "failed create newDocument", "err", err)
						}
						doc.Find("b").Each(func(i int, s *goquery.Selection) {
							b, err := regexp.MatchString(name, s.Text())
							if err != nil {
								level.Error(c.cfg.Logger).Log("msg", "failed regexp", "err", err)
							}
							if b {
								s = s.Parent()
								r.Link = "http://" + s.Find("a").Text()
								r.Code = s.Find("code").Text()
								records = append(records, r)
							}
						})
					}
				}
			})
		}
	})
	err = c.cfg.Storage.Collect(records)
	if err != nil {
		return fmt.Errorf("failed collect all records: %v", err)
	}
	return nil
}
