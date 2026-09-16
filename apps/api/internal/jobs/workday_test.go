package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseWorkdayCareerURL(t *testing.T) {
	cases := []struct { raw, token string }{
		{"https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/US-CA/Senior-Engineer_JR123", "nvidia.wd5.myworkdayjobs.com|nvidia|NVIDIAExternalCareerSite"},
		{"https://example.wd1.myworkdaysite.com/External/jobs", "example.wd1.myworkdaysite.com|example|External"},
	}
	for _,tc:=range cases{got,ok:=parseWorkdayCareerURL(tc.raw);if !ok{t.Fatalf("%s: expected Workday discovery",tc.raw)};if got.BoardToken!=tc.token{t.Fatalf("%s: expected token %q, got %q",tc.raw,tc.token,got.BoardToken)};if !got.Monitorable||got.SourceType!="WORKDAY"{t.Fatalf("%s: unexpected discovery %+v",tc.raw,got)}}
}
func TestParseWorkdayCareerURLNeedsSite(t *testing.T){if _,ok:=parseWorkdayCareerURL("https://nvidia.wd5.myworkdayjobs.com");ok{t.Fatal("host-only Workday URL must not be promoted without an exact site")}}

func TestWorkdaySourceFetchUsesFullSnapshotAndFreshDetails(t *testing.T) {
	now:=time.Date(2026,9,8,12,0,0,0,time.UTC);detailCalls:=0
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){switch r.URL.Path{
	case "/wday/cxs/acme/External/jobs":var request struct{Limit int `json:"limit"`;Offset int `json:"offset"`};if err:=json.NewDecoder(r.Body).Decode(&request);err!=nil{t.Fatal(err)};if request.Limit!=20||request.Offset!=0{t.Fatalf("unexpected list request %+v",request)};w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"total":2,"jobPostings":[{"title":"Senior Backend Engineer","externalPath":"/job/Austin-TX/Senior-Backend-Engineer_R123","locationsText":"Austin, TX","postedOn":"Posted 2 Days Ago","bulletFields":["R123"]},{"title":"Old Engineer","externalPath":"/job/Austin-TX/Old-Engineer_R100","locationsText":"Austin, TX","postedOn":"Posted 30 Days Ago","bulletFields":["R100"]}]}`))
	case "/wday/cxs/acme/External/job/Senior-Backend-Engineer_R123":detailCalls++;w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"jobPostingInfo":{"title":"Senior Backend Engineer","jobReqId":"R123","jobPostingId":"Senior-Backend-Engineer_R123","jobDescription":"<p>Build distributed Java and Go services.</p>","startDate":"2026-09-06","location":"Austin, Texas, United States of America","timeType":"Full time","remoteType":"Hybrid","jobRequisitionLocation":{"country":{"alpha2Code":"US","descriptor":"United States of America"}}}}`))
	default:t.Fatalf("unexpected request %s %s",r.Method,r.URL.Path)}}));defer server.Close()
	source,err:=NewWorkdaySource("acme.wd1.myworkdayjobs.com|acme|External");if err!=nil{t.Fatal(err)};source.BaseURL=server.URL;source.now=func()time.Time{return now};source.DetailMaxAge=7*24*time.Hour
	jobs,_,err:=source.Fetch(context.Background(),nil);if err!=nil{t.Fatal(err)};if detailCalls!=1{t.Fatalf("expected one fresh detail request, got %d",detailCalls)};if len(jobs)!=1||jobs[0].ExternalID!="R123"||jobs[0].RemoteType!="hybrid"{t.Fatalf("unexpected jobs %+v",jobs)};if jobs[0].Description!="Build distributed Java and Go services."{t.Fatalf("unexpected description %q",jobs[0].Description)};seen:=source.SeenExternalIDs();if len(seen)!=2||seen[0]!="R123"||seen[1]!="R100"{t.Fatalf("unexpected full snapshot ids %+v",seen)}
}

func TestWorkdaySourceFetchSkipsListingWhoseDetailDisappears(t *testing.T) {
	now:=time.Date(2026,9,16,12,0,0,0,time.UTC)
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){switch r.URL.Path{
	case "/wday/cxs/acme/External/jobs":w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"total":2,"jobPostings":[{"title":"Disappearing Job","externalPath":"/job/Disappearing_R404","postedOn":"Posted Today","bulletFields":["R404"]},{"title":"Live Job","externalPath":"/job/Live_R200","postedOn":"Posted Today","bulletFields":["R200"]}]}`))
	case "/wday/cxs/acme/External/job/Disappearing_R404":http.NotFound(w,r)
	case "/wday/cxs/acme/External/job/Live_R200":w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"jobPostingInfo":{"title":"Live Job","jobDescription":"Build services.","startDate":"2026-09-16","location":"Austin, TX"}}`))
	default:t.Fatalf("unexpected request %s",r.URL.Path)}}));defer server.Close()
	source,err:=NewWorkdaySource("acme.wd1.myworkdayjobs.com|acme|External");if err!=nil{t.Fatal(err)};source.BaseURL=server.URL;source.now=func()time.Time{return now}
	jobs,_,err:=source.Fetch(context.Background(),nil);if err!=nil{t.Fatalf("detail 404 must not invalidate complete listing snapshot: %v",err)};if len(jobs)!=1||jobs[0].ExternalID!="R200"{t.Fatalf("expected remaining live job, got %+v",jobs)};seen:=source.SeenExternalIDs();if len(seen)!=2||seen[0]!="R404"||seen[1]!="R200"{t.Fatalf("listing snapshot must retain both IDs, got %+v",seen)}
}

func TestParseWorkdayPostedOn(t *testing.T){now:=time.Date(2026,9,8,12,0,0,0,time.UTC);got:=parseWorkdayPostedOn("Posted Yesterday",now);if got==nil||!got.Equal(now.Add(-24*time.Hour)){t.Fatalf("unexpected yesterday timestamp %v",got)};got=parseWorkdayPostedOn("Posted 3 Days Ago",now);if got==nil||!got.Equal(now.Add(-72*time.Hour)){t.Fatalf("unexpected relative timestamp %v",got)}}
