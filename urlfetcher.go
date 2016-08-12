package urlfetcher

import (
  //"html/template"
  "net/http"
  "fmt"
  "time"
  "appengine"
  "appengine/datastore"
  "io/ioutil"
  //"math"
  //"bufio"
  //"os"
  //"reflect"
  //"golang.org/x/net/context"
  "regexp"
  "strings"
  //"encoding/json"
  //"appengine/user"
  "appengine/urlfetch"

)

type HttpResponse struct {
  url      string
  response *http.Response
  err      error
}

type Result struct {
  url      string
  detail   string
  status   string
}

func ProcessUrls(ctx appengine.Context, urls []string, matchContains []string) string {
  numIterations := int(len(urls)/10) + 1
  endIndex := 0
  respStringArray := make ([]string,0 )
  for x := 0; x < numIterations; x++ {
    endIndex = ((x+1) * 10)
    if endIndex > len(urls) {
      endIndex = len(urls)
    }
    theseUrls := urls[x*10:endIndex]
    results := AsyncHttpGets(ctx, theseUrls, matchContains)
    for _ , result := range results {
      respStringArray = append(respStringArray, result)
    }
  }
  return strings.Join(respStringArray, "\n")
}

func AsyncHttpGets(ctx appengine.Context, urls []string, matchContains []string) []string {
  re := regexp.MustCompile(`("http(s)?:\/\/.*?")`)
  ch := make(chan *HttpResponse)

  client := urlfetch.Client(ctx)

  //timeout := time.Duration(10 * time.Second)
  //client := http.Client{
  //    Timeout: timeout,
  //}
  responses := make ([]string,0 )

  for _, url := range urls {
      go func(url string) {
          resp, err := client.Get(url)
          ch <- &HttpResponse{url, resp, err}
      }(url)
  }
  respCtr := 0
  for {
      select {
      case r := <-ch:
          if r.err != nil {
              responses = append(responses, r.url + ","+ r.err.Error() + ",0")
          } else {
            res_text, err := ioutil.ReadAll(r.response.Body)
            if err != nil {
              responses = append(responses, r.url +",error webpage,0")
            } else {
              foundStrings := re.FindAllString(string(res_text), -1)
               for _, foundString := range(foundStrings) {
                 for _, matchContain := range(matchContains) {
                   if strings.Contains(foundString, matchContain) == true {
                     responses = append(responses, r.url + "," +foundString + ",1")
                   }
                 }
               }
            }
          }
          respCtr += 1
          if respCtr == len(urls) {
            return responses
          }
      }
  }
  return responses
}


type UrlFetch struct {
  RequestUid string
  RequestUrls string `datastore:",noindex"`
  RequestContains string `datastore:",noindex"`
  Result string `datastore:",noindex"`
  Completed string
  Date    time.Time
}

func init() {
  http.HandleFunc("/urlfetch", handleFetch)
  http.HandleFunc("/processfetch", processFetch)
}

func processFetch(w http.ResponseWriter, r *http.Request) {
  ctx := appengine.NewContext(r)
  q := datastore.NewQuery("UrlFetch").Filter("Completed =", "n").Order("-Date").Limit(1)
  var queryRes []*UrlFetch
  if _, err := q.GetAll(ctx, &queryRes); err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }
  for _, urlFetchInst := range(queryRes) {
    urlArray := strings.Split(urlFetchInst.RequestUrls, ",")
    containsArray := strings.Split(urlFetchInst.RequestContains, ",")
    resultString := ProcessUrls(ctx, urlArray, containsArray)
    urlFetchInst.Completed = "y"
    urlFetchInst.Result = resultString
    key := datastore.NewKey(
            ctx,
            "UrlFetch",
            urlFetchInst.RequestUid,
            0,
            nil,
    )
    datastore.Put(ctx, key, urlFetchInst)
  }
}

func handleFetch(w http.ResponseWriter, r *http.Request) {
  ctx := appengine.NewContext(r)
  requestUid := r.FormValue("request_uid")
  key := datastore.NewKey(
          ctx,
          "UrlFetch",
          requestUid,
          0,
          nil,
  )
  if r.Method == "GET" {
    var urlFetch UrlFetch
    err := datastore.Get(ctx, key, &urlFetch)
      if err != nil {
        w.WriteHeader(404)
        fmt.Fprintf(w, "error getting datastore item")
      } else {
          if urlFetch.Completed == "y" {
            err = datastore.Delete(ctx, key)
              if err != nil {
                w.WriteHeader(404)
                fmt.Fprintf(w, "error deleting")
              } else {
                fmt.Fprintf(w, urlFetch.Result)
              }
            } else {
              w.WriteHeader(204)
              fmt.Fprintf(w, "not processed yet")
          }
      }
    } else {
      requestUrls := r.FormValue("request_urls")
      requestContains := r.FormValue("request_contains")
        urlFetchInst := &UrlFetch{
                RequestUid: requestUid,
                RequestUrls: requestUrls,
                RequestContains: requestContains,
                Result:  "",
                Completed: "n",
                Date:  time.Now(),
        }
      if _, err := datastore.Put(ctx, key, urlFetchInst); err != nil {
              fmt.Fprintf(w, "datastore error")
      } else {
        fmt.Fprintf(w, "%q added datstore item", requestUid)
      }
  }
}
