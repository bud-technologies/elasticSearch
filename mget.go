// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MgetService allows to get multiple documents based on an index,
// type (optional) and id (possibly routing). The response includes
// a docs array with all the fetched documents, each element similar
// in structure to a document provided by the Get API.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-multi-get.html
// for details.
type MgetService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	preference   string
	realtime     *bool
	refresh      string
	routing      string
	storedFields []string
	items        []*MultiGetItem
	mgetCache    Cache
	cacheTime    time.Duration
}

// NewMgetService initializes a new Multi GET API request call.
func NewMgetService(client *Client) *MgetService {
	builder := &MgetService{
		client: client,
	}
	return builder
}

// Cache set a cache
func (s *MgetService) Cache(cache Cache, expiration time.Duration) *MgetService {
	s.mgetCache = cache
	s.cacheTime = expiration
	return s
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *MgetService) Pretty(pretty bool) *MgetService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *MgetService) Human(human bool) *MgetService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *MgetService) ErrorTrace(errorTrace bool) *MgetService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *MgetService) FilterPath(filterPath ...string) *MgetService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *MgetService) Header(name string, value string) *MgetService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *MgetService) Headers(headers http.Header) *MgetService {
	s.headers = headers
	return s
}

// Preference specifies the node or shard the operation should be performed
// on (default: random).
func (s *MgetService) Preference(preference string) *MgetService {
	s.preference = preference
	return s
}

// Refresh the shard containing the document before performing the operation.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-refresh.html
// for details.
func (s *MgetService) Refresh(refresh string) *MgetService {
	s.refresh = refresh
	return s
}

// Realtime specifies whether to perform the operation in realtime or search mode.
func (s *MgetService) Realtime(realtime bool) *MgetService {
	s.realtime = &realtime
	return s
}

// Routing is the specific routing value.
func (s *MgetService) Routing(routing string) *MgetService {
	s.routing = routing
	return s
}

// StoredFields is a list of fields to return in the response.
func (s *MgetService) StoredFields(storedFields ...string) *MgetService {
	s.storedFields = append(s.storedFields, storedFields...)
	return s
}

// Add an item to the request.
func (s *MgetService) Add(items ...*MultiGetItem) *MgetService {
	s.items = append(s.items, items...)
	return s
}

// Source returns the request body, which will be serialized into JSON.
func (s *MgetService) Source() (interface{}, error) {
	source := make(map[string]interface{})
	items := make([]interface{}, len(s.items))
	for i, item := range s.items {
		src, err := item.Source()
		if err != nil {
			return nil, err
		}
		items[i] = src
	}
	source["docs"] = items
	return source, nil
}

func (s *MgetService) getMd5Id() (string, error) {
	var md5Id string
	str, err := json.Marshal(*s)
	if err != nil {
		return md5Id, err
	}
	md5Id = str2md5(str)
	return md5Id, nil
}

// Do executes the request.
func (s *MgetService) Do(ctx context.Context) (*MgetResponse, error) {
	if s.mgetCache != nil {
		result, getErr := s.mgetCache.Get(ctx, MGet)
		if getErr != nil && !getErr.IsTimeOutErr() && !getErr.IsNotFoundErr() {
			return nil, getErr
		} else if getErr == nil && result != "" {
			rsp := &MgetResponse{}
			err := json.Unmarshal([]byte(result), rsp)
			if err != nil {
				return nil, err
			}
			return rsp, nil
		}
	}

	// Build url
	path := "/_mget"

	params := url.Values{}
	if v := s.pretty; v != nil {
		params.Set("pretty", fmt.Sprint(*v))
	}
	if v := s.human; v != nil {
		params.Set("human", fmt.Sprint(*v))
	}
	if v := s.errorTrace; v != nil {
		params.Set("error_trace", fmt.Sprint(*v))
	}
	if len(s.filterPath) > 0 {
		params.Set("filter_path", strings.Join(s.filterPath, ","))
	}
	if s.realtime != nil {
		params.Add("realtime", fmt.Sprintf("%v", *s.realtime))
	}
	if s.preference != "" {
		params.Add("preference", s.preference)
	}
	if s.refresh != "" {
		params.Add("refresh", s.refresh)
	}
	if s.routing != "" {
		params.Set("routing", s.routing)
	}
	if len(s.storedFields) > 0 {
		params.Set("stored_fields", strings.Join(s.storedFields, ","))
	}

	// Set body
	body, err := s.Source()
	if err != nil {
		return nil, err
	}

	indexList := make([]string, 0)
	for _, item := range s.items {
		if item != nil {
			indexList = append(indexList, item.index)
		}
	}

	// Get response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "GET",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
		Info: &RequestInfo{
			RequestType: MGet,
			Index:       indexList,
		},
	})
	if err != nil {
		return nil, err
	}

	// Return result
	ret := new(MgetResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	if s.mgetCache != nil {
		retStr, err := json.Marshal(*ret)
		if err != nil {
			s.client.errorlog.Printf("Marshal ret err", err)
		}
		err = s.mgetCache.Put(ctx, MGet, string(retStr), s.cacheTime)
		if err != nil && s.client.errorlog != nil {
			s.client.errorlog.Printf("put cache err", err)
		}
	}
	return ret, nil
}

// -- Multi Get Item --

// MultiGetItem is a single document to retrieve via the MgetService.
type MultiGetItem struct {
	index        string
	typ          string
	id           string
	routing      string
	storedFields []string
	version      *int64 // see org.elasticsearch.common.lucene.uid.Versions
	versionType  string // see org.elasticsearch.index.VersionType
	fsc          *FetchSourceContext
}

// NewMultiGetItem initializes a new, single item for a Multi GET request.
func NewMultiGetItem() *MultiGetItem {
	return &MultiGetItem{}
}

// Index specifies the index name.
func (item *MultiGetItem) Index(index string) *MultiGetItem {
	item.index = index
	return item
}

// Type specifies the type name.
func (item *MultiGetItem) Type(typ string) *MultiGetItem {
	item.typ = typ
	return item
}

// Id specifies the identifier of the document.
func (item *MultiGetItem) Id(id string) *MultiGetItem {
	item.id = id
	return item
}

// Routing is the specific routing value.
func (item *MultiGetItem) Routing(routing string) *MultiGetItem {
	item.routing = routing
	return item
}

// StoredFields is a list of fields to return in the response.
func (item *MultiGetItem) StoredFields(storedFields ...string) *MultiGetItem {
	item.storedFields = append(item.storedFields, storedFields...)
	return item
}

// Version can be MatchAny (-3), MatchAnyPre120 (0), NotFound (-1),
// or NotSet (-2). These are specified in org.elasticsearch.common.lucene.uid.Versions.
// The default in Elasticsearch is MatchAny (-3).
func (item *MultiGetItem) Version(version int64) *MultiGetItem {
	item.version = &version
	return item
}

// VersionType can be "internal", "external", "external_gt", or "external_gte".
// See org.elasticsearch.index.VersionType in Elasticsearch source.
// It is "internal" by default.
func (item *MultiGetItem) VersionType(versionType string) *MultiGetItem {
	item.versionType = versionType
	return item
}

// FetchSource allows to specify source filtering.
func (item *MultiGetItem) FetchSource(fetchSourceContext *FetchSourceContext) *MultiGetItem {
	item.fsc = fetchSourceContext
	return item
}

// Source returns the serialized JSON to be sent to Elasticsearch as
// part of a MultiGet search.
func (item *MultiGetItem) Source() (interface{}, error) {
	source := make(map[string]interface{})

	source["_id"] = item.id

	if item.index != "" {
		source["_index"] = item.index
	}
	if item.typ != "" {
		source["_type"] = item.typ
	}
	if item.fsc != nil {
		src, err := item.fsc.Source()
		if err != nil {
			return nil, err
		}
		source["_source"] = src
	}
	if item.routing != "" {
		source["routing"] = item.routing
	}
	if len(item.storedFields) > 0 {
		source["stored_fields"] = strings.Join(item.storedFields, ",")
	}
	if item.version != nil {
		source["version"] = fmt.Sprintf("%d", *item.version)
	}
	if item.versionType != "" {
		source["version_type"] = item.versionType
	}

	return source, nil
}

// -- Result of a Multi Get request.

// MgetResponse is the outcome of a Multi GET API request.
type MgetResponse struct {
	Docs []*GetResult `json:"docs,omitempty"`
}
