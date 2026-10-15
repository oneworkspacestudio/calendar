package app

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/oneworkspacestudio/calendar/shared"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DAVItem struct {
	IsCalendar bool   `json:"-"`
	Href       string `json:"href"`
	ETag       string `json:"etag"`
	Data       string `json:"data"`
	Name       string `json:"name,omitempty"`
	Color      string `json:"color,omitempty"`
	ReadOnly   bool   `json:"readOnly,omitempty"`
}
type multistatus struct {
	Responses []struct {
		Href     string `xml:"href"`
		Propstat []struct {
			Status string `xml:"status"`
			Prop   struct {
				ETag         string `xml:"getetag"`
				CalendarData string `xml:"calendar-data"`
				AddressData  string `xml:"address-data"`
				DisplayName  string `xml:"displayname"`
				Color        string `xml:"calendar-color"`
				ResourceType struct {
					Calendar    *struct{} `xml:"calendar"`
					AddressBook *struct{} `xml:"addressbook"`
				} `xml:"resourcetype"`
				Privileges struct {
					Write *struct{} `xml:"privilege>write"`
					Read  *struct{} `xml:"privilege>read"`
				} `xml:"current-user-privilege-set"`
			} `xml:"prop"`
		} `xml:"propstat"`
	} `xml:"response"`
}

func (a *App) dav(r *http.Request, s *Session, method, path, body, contentType, etag string) (*http.Response, error) {
	c, _, e := shared.Load(a.root)
	if e != nil {
		return nil, errors.New("Shared backend configuration is unavailable")
	}
	req, e := http.NewRequestWithContext(r.Context(), method, shared.EngineURL(c)+path, strings.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", s.auth())
	req.Header.Set("Content-Type", contentType)
	if method == "PROPFIND" {
		req.Header.Set("Depth", "1")
	}
	if method == "PUT" || method == "DELETE" {
		if etag == "*" && method == "PUT" {
			req.Header.Set("If-None-Match", "*")
		} else if etag != "" {
			req.Header.Set("If-Match", etag)
		} else {
			return nil, errors.New("refresh the resource before changing it; its version is missing")
		}
	}
	return a.client.Do(req)
}
func parseDAV(response *http.Response) ([]DAVItem, error) {
	defer response.Body.Close()
	if response.StatusCode != 207 {
		return nil, fmt.Errorf("Calendar service returned HTTP %d", response.StatusCode)
	}
	var result multistatus
	if e := xml.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&result); e != nil {
		return nil, errors.New("Calendar response could not be read")
	}
	items := []DAVItem{}
	for _, res := range result.Responses {
		item := DAVItem{Href: res.Href}
		good := false
		for _, prop := range res.Propstat {
			if !strings.Contains(prop.Status, " 200 ") {
				continue
			}
			good = true
			p := prop.Prop
			if p.ETag != "" {
				item.ETag = p.ETag
			}
			if p.CalendarData != "" {
				item.Data = p.CalendarData
			}
			if p.AddressData != "" {
				item.Data = p.AddressData
			}
			if p.DisplayName != "" {
				item.Name = p.DisplayName
			}
			if p.Color != "" {
				item.Color = p.Color
			}
			if p.ResourceType.Calendar != nil {
				item.IsCalendar = true
			}
			if p.ResourceType.Calendar != nil || p.ResourceType.AddressBook != nil {
				if item.Name == "" {
					item.Name = "My calendar"
				}
			}
		}
		if good {
			items = append(items, item)
		}
	}
	return items, nil
}
func (a *App) home(s *Session, contacts bool) string {
	kind := "cal"
	if contacts {
		kind = "card"
	}
	return "/dav/" + kind + "/" + url.PathEscape(s.Subject+"@"+a.cfg.Domain) + "/"
}
func (a *App) calendars(w http.ResponseWriter, r *http.Request) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	if a.demo {
		a.demoMu.Lock()
		defer a.demoMu.Unlock()
		writeJSON(w, a.demoCalendars)
		return
	}
	body := `<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:a="http://apple.com/ns/ical/"><d:prop><d:displayname/><d:resourcetype/><a:calendar-color/><d:current-user-privilege-set/></d:prop></d:propfind>`
	response, e := a.dav(r, s, "PROPFIND", a.home(s, false), body, "application/xml", "")
	if e != nil {
		fail(w, e)
		return
	}
	items, e := parseDAV(response)
	if e != nil {
		fail(w, e)
		return
	}
	result := []DAVItem{}
	for _, item := range items {
		if item.IsCalendar {
			if !strings.HasSuffix(item.Href, "/") {
				item.Href += "/"
			}
			if item.Color == "" {
				item.Color = "#1a73e8"
			}
			result = append(result, item)
		}
	}
	writeJSON(w, result)
}
func (a *App) createCalendar(w http.ResponseWriter, r *http.Request) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	var in struct{ Name, Color string }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil || len(in.Name) == 0 || len(in.Name) > 200 {
		http.Error(w, "Enter a calendar name", 400)
		return
	}
	path := a.home(s, false) + fmt.Sprint(time.Now().UnixNano()) + "/"
	if a.demo {
		a.demoMu.Lock()
		a.demoCalendars = append(a.demoCalendars, DAVItem{Href: path, Name: in.Name, Color: in.Color})
		a.demoMu.Unlock()
		writeJSON(w, map[string]string{"href": path})
		return
	}
	body := `<c:mkcalendar xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:d="DAV:" xmlns:a="http://apple.com/ns/ical/"><d:set><d:prop><d:displayname>` + html.EscapeString(in.Name) + `</d:displayname><a:calendar-color>` + html.EscapeString(in.Color) + `</a:calendar-color></d:prop></d:set></c:mkcalendar>`
	res, e := a.dav(r, s, "MKCALENDAR", path, body, "application/xml", "")
	if e != nil {
		fail(w, e)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		fail(w, fmt.Errorf("Calendar creation returned HTTP %d", res.StatusCode))
		return
	}
	writeJSON(w, map[string]string{"href": path})
}
func (a *App) events(w http.ResponseWriter, r *http.Request)   { a.resources(w, r, false) }
func (a *App) contacts(w http.ResponseWriter, r *http.Request) { a.resources(w, r, true) }
func (a *App) resources(w http.ResponseWriter, r *http.Request, contacts bool) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	if a.demo {
		a.demoMu.Lock()
		defer a.demoMu.Unlock()
		items := []DAVItem{}
		for _, item := range a.demoData {
			if strings.HasSuffix(item.Href, ".vcf") == contacts {
				items = append(items, item)
			}
		}
		writeJSON(w, items)
		return
	}
	prefix := "/dav/cal/"
	path := r.URL.Query().Get("calendar")
	if contacts {
		prefix = "/dav/card/"
		path = a.home(s, true) + "default/"
	}
	if path == "" {
		path = a.home(s, false) + "default/"
	}
	path, e := davPath(path, prefix)
	if e != nil {
		fail(w, e)
		return
	}
	body := `<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:getetag/><c:calendar-data/></d:prop><c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"/></c:comp-filter></c:filter></c:calendar-query>`
	if contacts {
		body = `<c:addressbook-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:carddav"><d:prop><d:getetag/><c:address-data/></d:prop><c:filter/></c:addressbook-query>`
	}
	response, e := a.dav(r, s, "REPORT", path, body, "application/xml", "")
	if e != nil {
		fail(w, e)
		return
	}
	items, e := parseDAV(response)
	if e != nil {
		fail(w, e)
		return
	}
	writeJSON(w, items)
}
func (a *App) putEvent(w http.ResponseWriter, r *http.Request)    { a.writeResource(w, r, false, false) }
func (a *App) deleteEvent(w http.ResponseWriter, r *http.Request) { a.writeResource(w, r, false, true) }
func (a *App) putContact(w http.ResponseWriter, r *http.Request)  { a.writeResource(w, r, true, false) }
func (a *App) deleteContact(w http.ResponseWriter, r *http.Request) {
	a.writeResource(w, r, true, true)
}
func (a *App) writeResource(w http.ResponseWriter, r *http.Request, contacts, remove bool) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	var in DAVItem
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&in) != nil {
		http.Error(w, "Invalid resource", 400)
		return
	}
	prefix := "/dav/cal/"
	extension := ".ics"
	contentType := "text/calendar; charset=utf-8"
	if contacts {
		prefix = "/dav/card/"
		extension = ".vcf"
		contentType = "text/vcard; charset=utf-8"
	}
	path, e := davPath(in.Href, prefix)
	if e != nil || !strings.HasSuffix(path, extension) {
		http.Error(w, "Invalid resource path", 400)
		return
	}
	if in.ETag == "" {
		http.Error(w, "Refresh before saving changes", 428)
		return
	}
	if !remove && !strings.HasPrefix(in.Data, "BEGIN:V") {
		http.Error(w, "Invalid calendar/contact data", 400)
		return
	}
	if a.demo {
		a.demoMu.Lock()
		defer a.demoMu.Unlock()
		old, exists := a.demoData[in.Href]
		if (in.ETag == "*" && exists) || (in.ETag != "*" && (!exists || old.ETag != in.ETag)) {
			http.Error(w, "Changed elsewhere; refresh before saving", 412)
			return
		}
		if remove {
			delete(a.demoData, in.Href)
		} else {
			in.ETag = fmt.Sprintf(`"%d"`, time.Now().UnixNano())
			a.demoData[in.Href] = in
		}
		writeJSON(w, map[string]string{"etag": in.ETag})
		return
	}
	method := "PUT"
	if remove {
		method = "DELETE"
	}
	response, e := a.dav(r, s, method, path, in.Data, contentType, in.ETag)
	if e != nil {
		fail(w, e)
		return
	}
	defer response.Body.Close()
	if response.StatusCode == 412 {
		http.Error(w, "This item changed elsewhere. Refresh before saving so those changes are preserved.", 412)
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		fail(w, fmt.Errorf("Service returned HTTP %d; changes were not confirmed", response.StatusCode))
		return
	}
	etag := response.Header.Get("ETag")
	if !remove && etag == "" {
		check, e := a.dav(r, s, "GET", path, "", "", "")
		if e == nil {
			etag = check.Header.Get("ETag")
			io.Copy(io.Discard, io.LimitReader(check.Body, 2<<20))
			check.Body.Close()
		}
	}
	writeJSON(w, map[string]string{"etag": etag})
}
func (a *App) seedDemo() {
	a.demoCalendars = []DAVItem{{Href: "/dav/cal/demo/default/", Name: "Alex Morgan", Color: "#1a73e8"}, {Href: "/dav/cal/demo/personal/", Name: "Personal", Color: "#0b8043"}, {Href: "/dav/cal/demo/work/", Name: "Work", Color: "#8e24aa"}}
	today := time.Now()
	start := today.AddDate(0, 0, -(int(today.Weekday())+6)%7)
	names := []string{"Design review", "Focus time", "Product sync", "Lunch with Sam", "Team planning", "Project catch-up", "Weekly review"}
	colors := []string{"default", "work", "default", "personal", "work", "personal", "default"}
	for i, name := range names {
		date := time.Date(start.Year(), start.Month(), start.Day()+i, 9+i%4, 0, 0, 0, time.Local)
		uid := fmt.Sprintf("demo-%d", i)
		href := "/dav/cal/demo/" + colors[i] + "/" + uid + ".ics"
		data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//One Workspace//Calendar//EN\r\nBEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTART:" + date.UTC().Format("20060102T150405Z") + "\r\nDTEND:" + date.Add(time.Hour).UTC().Format("20060102T150405Z") + "\r\nSUMMARY:" + name + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
		a.demoData[href] = DAVItem{Href: href, ETag: `"demo-1"`, Data: data}
	}
	a.demoData["/dav/card/demo/default/alex.vcf"] = DAVItem{Href: "/dav/card/demo/default/alex.vcf", ETag: `"demo-1"`, Data: "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:alex\r\nFN:Sam Taylor\r\nEMAIL:sam@example.test\r\nTEL:+44 20 7946 0123\r\nEND:VCARD\r\n"}
}
