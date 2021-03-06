package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	elastic "gopkg.in/olivere/elastic.v5"

	"github.com/levilovelock/magitrak/models"
	_ "github.com/levilovelock/magitrak/routers"
	"github.com/levilovelock/magitrak/tests/common"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/suite"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
)

func init() {
	// We initialise the beego here because the SuiteSetup changes dir to testify home
	_, file, _, _ := runtime.Caller(1)
	apppath, _ := filepath.Abs(filepath.Dir(filepath.Join(file, ".."+string(filepath.Separator))) + "/tests")
	beego.TestBeegoInit(apppath)
}

type MatchFuncTestSuite struct {
	suite.Suite
}

func TestMatchTestSuite(t *testing.T) {
	suite.Run(t, new(MatchFuncTestSuite))
}

func (s *MatchFuncTestSuite) SetupSuite() {
	dbAddress := beego.AppConfig.String("modelORMPrepopulatedAdress")
	dbType := beego.AppConfig.String("modelORMdb")

	dbErr := orm.RegisterDataBase("default", dbType, dbAddress, 30)
	if dbErr != nil {
		beego.Error(dbErr)
	}

	// Cleanse the ES data
	ctx := context.Background()
	client, err := elastic.NewClient(elastic.SetBasicAuth("elastic", "changeme"))
	if err != nil {
		panic(err)
	}
	client.DeleteIndex(models.ELASTIC_INDEX).Do(ctx)
	client.CreateIndex(models.ELASTIC_INDEX).Do(ctx)
}

func (s *MatchFuncTestSuite) TestMatchPOSTWithInvalidJSONReturns400() {
	body := []byte(`{"m"___,,L"'...aalidpassword"}`)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(400, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchPOSTWithDifferentUserIdInMatchThanSessionReturns400() {
	body := []byte(`{"userid": 4}`)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(400, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchPOSTValidUserIdAndMatchReturns200() {
	bodyObject := getValidMatch()
	body, _ := json.Marshal(bodyObject)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(200, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchGETNoLoginReturns401() {
	r, _ := http.NewRequest("GET", "/v1/match/1", nil)
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(302, w.Code)
	s.Assert().Equal("/v1/auth/unauthorised", w.Header().Get("Location"))
}

func (s *MatchFuncTestSuite) TestMatchPOSTNoPlayerDeckReturns400() {
	bodyObject := getValidMatch()
	bodyObject.PlayerDeck = ""

	body, _ := json.Marshal(bodyObject)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(400, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchPOSTNoOpponentDeckReturns400() {
	bodyObject := getValidMatch()
	bodyObject.OpponentDeck = ""

	body, _ := json.Marshal(bodyObject)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(400, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchPOSTNoDateReturns400() {
	bodyObject := getValidMatch()
	bodyObject.Date = time.Time{}

	body, _ := json.Marshal(bodyObject)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(400, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchGETInvalidIdReturns404() {
	r, _ := http.NewRequest("GET", "/v1/match/1234567", nil)
	w := httptest.NewRecorder()
	r.AddCookie(common.GetValidLoggedInSessionCookie())

	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(404, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchGETDifferentUserIDReturns400() {
	bodyObject := getValidMatch()
	body, _ := json.Marshal(bodyObject)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	// Retrieve matchId
	type MatchResult struct{ MatchId string }
	matchResult := &MatchResult{}
	json.Unmarshal([]byte(w.Body.String()), matchResult)

	r, _ = http.NewRequest("GET", "/v1/match/"+matchResult.MatchId, bytes.NewBuffer([]byte{}))
	w = httptest.NewRecorder()
	r.AddCookie(common.GetValidLoggedInSessionCookieOtherUser())

	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(400, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchInsertRetrievalASuccess() {
	testMatches := []models.Match{
		models.Match{
			UserId:           common.SESSION_USER_ID,
			Date:             time.Now(),
			PlayerDeck:       "jund",
			OpponentDeck:     "Titan",
			Win:              true,
			Reason:           "hand disruption + goyf",
			Sideboard:        false,
			PlayFirst:        true,
			StartingHandSize: 7,
			LandsInOpener:    2,
			OpponentName:     "killah31",
			Notes:            "na",
		}, models.Match{
			UserId:           common.SESSION_USER_ID,
			Date:             time.Now().Add(time.Hour * 195),
			PlayerDeck:       "twin",
			OpponentDeck:     "burn",
			Win:              false,
			Reason:           "mana screwed",
			Sideboard:        true,
			PlayFirst:        true,
			StartingHandSize: 6,
			LandsInOpener:    1,
			OpponentName:     "mangomaster",
			Notes:            "had double serum visions, still no land in top 8 cards!",
		}, models.Match{
			UserId:           common.SESSION_USER_ID,
			Date:             time.Now().Add(time.Hour * 543),
			PlayerDeck:       "ad nauseam",
			OpponentDeck:     "GR tron",
			Win:              true,
			Reason:           "no interaction, EZ",
			Sideboard:        false,
			PlayFirst:        false,
			StartingHandSize: 7,
			LandsInOpener:    4,
			OpponentName:     "tehpwnerer",
			Notes:            "always so EZ",
		},
	}

	for _, tm := range testMatches {
		body, _ := json.Marshal(tm)

		r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.AddCookie(common.GetValidLoggedInSessionCookie())
		beego.BeeApp.Handlers.ServeHTTP(w, r)

		s.Assert().Equal(200, w.Code)

		// Parse match result for Match ID
		type MatchResult struct{ MatchId string }
		matchResult := &MatchResult{}
		json.Unmarshal([]byte(w.Body.String()), matchResult)

		r, _ = http.NewRequest("GET", "/v1/match/"+matchResult.MatchId, nil)
		w = httptest.NewRecorder()
		r.AddCookie(common.GetValidLoggedInSessionCookie())
		beego.BeeApp.Handlers.ServeHTTP(w, r)

		s.Assert().Equal(200, w.Code)

		returnedMatch := &models.Match{}
		json.Unmarshal([]byte(w.Body.String()), returnedMatch)

		s.Assert().Equal(tm.UserId, returnedMatch.UserId)
		s.Assert().Equal(tm.Date, returnedMatch.Date)
		s.Assert().Equal(tm.LandsInOpener, returnedMatch.LandsInOpener)
		s.Assert().Equal(tm.Notes, returnedMatch.Notes)
		s.Assert().Equal(tm.OpponentDeck, returnedMatch.OpponentDeck)
		s.Assert().Equal(tm.OpponentName, returnedMatch.OpponentName)
		s.Assert().Equal(tm.PlayerDeck, returnedMatch.PlayerDeck)
		s.Assert().Equal(tm.PlayFirst, returnedMatch.PlayFirst)
		s.Assert().Equal(tm.Reason, returnedMatch.Reason)
		s.Assert().Equal(tm.Sideboard, returnedMatch.Sideboard)
		s.Assert().Equal(tm.Win, returnedMatch.Win)
		s.Assert().Equal(tm.StartingHandSize, returnedMatch.StartingHandSize)
	}
}

func (s *MatchFuncTestSuite) TestMatchDELETEInvalidIDReturns404() {
	r, _ := http.NewRequest("DELETE", "/v1/match/1234567", bytes.NewBuffer([]byte{}))
	w := httptest.NewRecorder()
	r.AddCookie(common.GetValidLoggedInSessionCookie())

	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(404, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchDELETEDifferentUserIDReturns400() {
	bodyObject := getValidMatch()
	body, _ := json.Marshal(bodyObject)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	// Retrieve matchId
	type MatchResult struct{ MatchId string }
	matchResult := &MatchResult{}
	json.Unmarshal([]byte(w.Body.String()), matchResult)

	r, _ = http.NewRequest("DELETE", "/v1/match/"+matchResult.MatchId, bytes.NewBuffer([]byte{}))
	w = httptest.NewRecorder()
	r.AddCookie(common.GetValidLoggedInSessionCookieOtherUser())

	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(400, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchInsertDeleteRetrieveSuccess() {
	bodyObject := getValidMatch()
	body, _ := json.Marshal(bodyObject)

	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	// Retrieve matchId
	type MatchResult struct{ MatchId string }
	matchResult := &MatchResult{}
	json.Unmarshal([]byte(w.Body.String()), matchResult)

	r, _ = http.NewRequest("DELETE", "/v1/match/"+matchResult.MatchId, bytes.NewBuffer([]byte{}))
	w = httptest.NewRecorder()
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(200, w.Code)
	s.Assert().Equal("\"delete success!\"", w.Body.String())

	r, _ = http.NewRequest("GET", "/v1/match/"+matchResult.MatchId, bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w = httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(404, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchesAllGETNoSession302() {
	r, _ := http.NewRequest("GET", "/v1/match", nil)
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)

	s.Assert().Equal(302, w.Code)
}

func (s *MatchFuncTestSuite) TestMatchAddTwoMatchesThenGetAllSuccess() {
	var match1, match2 *models.Match
	match1 = &models.Match{UserId: 1, PlayerDeck: "jund", OpponentDeck: "burn", Date: time.Now()}
	match2 = &models.Match{UserId: 1, PlayerDeck: "junk", OpponentDeck: "ur twin", Date: time.Now().Add(time.Hour * -99)}

	// Add match1
	body, _ := json.Marshal(match1)
	r, _ := http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)
	s.Assert().Equal(200, w.Code)
	beego.Debug("THE INSERT:", w.Body.String())

	// Add match2
	body, _ = json.Marshal(match2)
	r, _ = http.NewRequest("POST", "/v1/match", bytes.NewBuffer(body))
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	w = httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)
	s.Assert().Equal(200, w.Code)
	beego.Debug("THE INSERT:", w.Body.String())

	// Retry this test upto 10 times
	testsPass := false

	r, _ = http.NewRequest("GET", "/v1/match", nil)
	r.AddCookie(common.GetValidLoggedInSessionCookie())
	for i := 0; i < 10; i++ {
		w = httptest.NewRecorder()
		beego.BeeApp.Handlers.ServeHTTP(w, r)

		s.Assert().Equal(200, w.Code)

		// Parse results
		results := &[]models.Match{}
		json.Unmarshal(w.Body.Bytes(), results)

		// Find match 1 and match 2 in the results
		match1Found := false
		match2Found := false
		for _, match := range *results {
			s.Assert().True(match.UserId == common.SESSION_USER_ID)
			if match.Date == match1.Date && match.OpponentDeck == match1.OpponentDeck && match.PlayerDeck == match1.PlayerDeck {
				match1Found = true
			}
			if match.Date == match2.Date && match.OpponentDeck == match2.OpponentDeck && match.PlayerDeck == match2.PlayerDeck {
				match2Found = true
			}
		}

		if match1Found && match2Found {
			testsPass = true
			break
		} else {
			time.Sleep(time.Millisecond * 100)
		}
	}
	s.Assert().True(testsPass)
}

func getValidMatch() models.Match {
	return models.Match{UserId: 1, PlayerDeck: "burn", OpponentDeck: "bloom", Date: time.Now()}
}
