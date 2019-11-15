package collector

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/go-kit/kit/log/level"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-kit/kit/log"
)

var Collection = make(map[string]Record)

type Config struct {
	Logger      log.Logger
	Storage     Storage
	URLs        []string
	NameMarkets []string
}

type Storage interface {
	LoadCollect() (map[string]Record, error)
	Collect(records Record) error
}

type Collector struct {
	cfg *Config
}

type Record struct {
	ID     int    `db:"id"`
	PostID string `db:"post_id"`
	Market string `db:"market"`
	Link   string `db:"link"`
	Code   string `db:"code"`
}

func New(cfg *Config) (*Collector, error) {
	var err error
	if cfg.Storage == nil {
		return nil, errors.New("storage is empty")
	}
	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}
	if len(cfg.URLs) == 0 {
		return nil, errors.New("URLs must provide at least one url")
	}
	collector := &Collector{
		cfg: cfg,
	}
	level.Info(cfg.Logger).Log("msg", "create collector.")
	Collection, err = collector.cfg.Storage.LoadCollect()
	if err != nil {
		return nil, err
	}
	return collector, nil
}

type Filter struct {
	URL         string
	Deep        int
	PostID      *Attribute
	Code        *Attribute
	Description *Attribute
	Date        *Attribute
}

type Attribute struct {
	Deep    int
	Article string
	Attr    string
	Regexp  string
}

type Rec struct {
	Code        string
	Date        string
	PostID      string
	Description string
}

func (c *Collector) dima(d *goquery.Document, f *Filter) error {
	if c := f.Code; c.Deep == f.Deep {
		d.Find(c.Article).Each(func(i int, selection *goquery.Selection) {

		})
	}
	if dsc := f.Description; dsc != nil && dsc.Deep == f.Deep {
		str := regexp.MustCompile(dsc.Regexp).FindAllString(d.Find(dsc.Article).Text(), -1)
		fmt.Println("Description", str) // ЛитРес.{1,100}:
	}
	if f.Date != nil && f.Date.Deep == f.Deep {

	}
	return nil
}

func (c *Collector) Collector() error {
	begin := time.Now()
	level.Info(c.cfg.Logger).Log("msg", "collect records was start")
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
		if postID, exist := s.Attr("id"); exist {
			if _, ok := Collection[postID]; !ok {
				var r Record
				r.PostID = postID
				s.Find("a").Each(func(i int, s *goquery.Selection) {
					for _, name := range c.cfg.NameMarkets {
						existMarket, err := regexp.MatchString(name, s.Text())
						if err != nil {
							level.Error(c.cfg.Logger).Log("msg", "failed regexp", "err", err)
							continue
						}
						if existMarket {
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
								marketFound, err := regexp.MatchString(name, s.Text())
								if err != nil {
									level.Error(c.cfg.Logger).Log("msg", "failed regexp", "err", err)
								}
								if marketFound {
									s = s.Parent()
									r.Link = "http://" + s.Find("a").Text()
									s.Find("code").Each(func(i int, selection *goquery.Selection) {
										r.Code += fmt.Sprintf("%v: %v\t", i+1, selection.Text())
									})
									err = c.cfg.Storage.Collect(r)
									if err != nil {
										level.Error(c.cfg.Logger).Log("msg", "failed collect record", "err", err)
									}
								}
							})
						}
					}
				})
			}
		}
	})
	level.Info(c.cfg.Logger).Log("msg", "collect records was finished", "time elapsed", time.Since(begin))
	return nil
}
