package controllers

import (
	"encoding/json"

	"github.com/levilovelock/magitrak/models"

	"github.com/astaxie/beego"
)

// Operations about match
type MatchController struct {
	beego.Controller
}

// @router / [post]
func (m *MatchController) Create() {
	var match models.Match
	unmarshalErr := json.Unmarshal(m.Ctx.Input.RequestBody, &match)
	if unmarshalErr != nil {
		beego.Debug("Error unmarshalling request for Match POST:", unmarshalErr.Error())
		m.Abort("400")
	}

	validationErr := match.Validate()
	if validationErr != nil {
		beego.Debug("Error validating match for POST:", validationErr.Error())
		m.Abort("400")
	}

	magiSession := m.GetSession(models.SESSION_NAME)
	if magiSession == nil {
		beego.Debug("Failed to find valid session for match creation request")
		m.Abort("500")
	}

	if match.UserId != magiSession.(models.MagiSession).UserId {
		beego.Debug("Session userid does not match the userId in match data for match creation request")
		m.Abort("400")
	}

	matchId, insertErr := models.InsertMatch(match)
	if insertErr != nil {
		beego.Debug("Error inserting match into ElasticSearch:", insertErr.Error())
		m.Abort("500")
	}

	m.Data["json"] = map[string]string{"MatchId": matchId}
	m.ServeJSON()
}

// @router /:matchId [get]
func (m *MatchController) GetSingle() {
	matchId := m.Ctx.Input.Param(":matchId")
	match, err := models.GetOne(matchId)
	if err != nil {
		if err.Error() == models.NO_MATCH_FOUND_ERROR {
			m.Abort("404")
		} else {
			beego.Debug("Error finding match in GET for match id and err: ", matchId, err.Error())
			m.Abort("500")
		}
	} else {
		session := m.GetSession(models.SESSION_NAME).(models.MagiSession)
		if match.UserId != session.UserId {
			beego.Debug("Unauthorised GET request for match", matchId, "from session belonging to user", session.UserId)
			m.Abort("400")
		}
		m.Data["json"] = match
	}
	m.ServeJSON()
}

// @router /:matchId [delete]
func (m *MatchController) Delete() {
	matchId := m.Ctx.Input.Param(":matchId")

	match, findErr := models.GetOne(matchId)
	if findErr != nil {
		if findErr.Error() == models.NO_MATCH_FOUND_ERROR {
			m.Abort("404")
		} else {
			m.Abort("500")
		}
	}

	session := m.GetSession(models.SESSION_NAME).(models.MagiSession)
	if session.UserId != match.UserId {
		beego.Debug("Delete request for match belonging to different user found, user:", match.UserId)
		m.Abort("400")
	}

	deleteResult := models.Delete(matchId)

	if deleteResult != true {
		m.Abort("500")
	}

	m.Data["json"] = "delete success!"
	m.ServeJSON()
}

// @router / [get]
func (m *MatchController) GetAll() {
	userId := m.GetSession(models.SESSION_NAME).(models.MagiSession).UserId

	matches, err := models.GetAll(userId)
	if err != nil {
		beego.Debug("Error getting matches in GET all, err: ", err.Error())
		m.Abort("500")
	}

	m.Data["json"] = matches
	m.ServeJSON()
}
