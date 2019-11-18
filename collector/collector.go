package collector

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/kit/log/level"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-kit/kit/log"
)

var month = map[string]string{"январь": "January", "февраль": "February", "март": "March", "апрель": "April", "май": "May", "июнь": "June", "июль": "July", "август": "August", "сентябрь": "September", "октябрь": "October", "ноябрь": "November", "декабрь": "December"}

type Config struct {
	Logger  log.Logger
	Storage Storage
}

type Storage interface {
	Collect(records Record) error
}

type Collector struct {
	cfg *Config
}

func New(cfg *Config) (*Collector, error) {
	if cfg.Storage == nil {
		return nil, errors.New("storage is empty")
	}
	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}
	collector := &Collector{
		cfg: cfg,
	}
	level.Info(cfg.Logger).Log("msg", "create collector.")
	return collector, nil
}

type Record struct {
	ID          string `db:"id"`
	Code        string `db:"code"`
	Date        int64  `db:"date"`
	Link        string `db:"link"`
	PostID      string `db:"post_id"`
	Description string `db:"description"`
}

func (c *Collector) collect(d *goquery.Document) {
	d.Find("tbody").Each(func(i int, selection *goquery.Selection) {
		if i == 0 {
			selection.Find("tr").Each(func(i int, selection *goquery.Selection) {
				var r Record
				selection.Find("td").Each(func(i int, s *goquery.Selection) {
					switch i {
					case 0:
						var t time.Time
						var err error
						sep := strings.Split(s.Text(), " ")
						if sep[0] == "до" {
							t, err = time.Parse("02.01.2006", sep[1])
							if err != nil {
								level.Error(c.cfg.Logger).Log("msg", "failed parse time", "time", s.Text(), "err", err)
							}
						} else {
							t, err = time.Parse("2 January 2006", "30 "+month[sep[0]]+" "+sep[1])
							if err != nil {
								level.Error(c.cfg.Logger).Log("msg", "failed parse time", "time", s.Text(), "err", err)
							}
						}
						r.Date = t.Unix()
					case 1:
						r.Code = s.Text()
						if r.Code != "[автокод]" {
							r.Code = strings.Join(regexp.MustCompile(`[aA-zZ0-9]{1,100}`).FindAllString(r.Code, -1), " ")
						}
						r.Link, _ = s.Find("a").Attr("href")
						r.Link = strings.Replace(r.Link, "https://li.lovikod.ru", "https://www.litres.ru", -1)
						r.Link = strings.TrimSuffix(r.Link, "?lfrom=342676429")
					case 2:
						r.Description = s.Text()
					}
				})
				err := c.cfg.Storage.Collect(r)
				if err != nil {
					level.Error(c.cfg.Logger).Log("msg", "failed create record", "err", err)
				}
			})
		}
	})
}

type ConditionQuery struct {
	URI string
}

func (c *Collector) Collect(cq ConditionQuery) error {
	begin := time.Now()
	level.Info(c.cfg.Logger).Log("msg", "collect", "url", cq.URI)
	resp, err := http.Get(cq.URI)
	if err != nil {
		return fmt.Errorf("failed get request url: %s, reason: %v", cq.URI, err)

	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("failed request got status code %v", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed create newDocument: %v", err)
	}

	c.collect(doc)
	level.Info(c.cfg.Logger).Log("msg", "collect records was finished", "time elapsed", time.Since(begin))
	return nil
}
