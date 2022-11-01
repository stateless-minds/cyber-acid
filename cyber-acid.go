package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/foolin/mixer"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
	"github.com/mitchellh/mapstructure"
	shell "github.com/stateless-minds/go-ipfs-api"
)

const dbAddressIssue = "/orbitdb/bafyreihjgftbxrabuhfjn7diwtlz67hnhu2jsivds5arhqqb3xybov7eku/issue"

const dbAddressCitizenReputation = "/orbitdb/bafyreide5xex6dwtdg45eserwx2ib2cjeqpfu4hcjnik26hzvl525rwqoy/citizen_reputation"

const typeShortage = "shortage"

const (
	NotificationSuccess NotificationStatus = "positive"
	NotificationInfo    NotificationStatus = "info"
	NotificationWarning NotificationStatus = "warning"
	NotificationDanger  NotificationStatus = "negative"
	SuccessHeader                          = "Success"
	ErrorHeader                            = "Error"
)

const (
	asideTitleCreate = "Suggest Solution"
	asideTitleList   = "List Solutions"
)

// pubsub is a component that does a simple pubsub on ipfs. A component is a
// customizable, independent, and reusable UI element. It is created by
// embedding app.Compo into a struct.
type acid struct {
	app.Compo
	topic                  string
	sh                     *shell.Shell
	sub                    *shell.PubSubSubscription
	citizenID              string
	issues                 []Issue
	ranks                  []CitizenReputation
	delegates              []Delegate
	currentIssueInSlice    int
	currentSolutionInSlice int
	Solutions              []Solution
	currentSolutionDesc    string
	notifications          map[string]notification
	notificationID         int
	AsideTitle             string
}

type NotificationStatus string

type notification struct {
	id      int
	status  string
	header  string
	message string
}

type Issue struct {
	ID        string     `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`
	Type      string     `mapstructure:"type" json:"type" validate:"uuid_rfc4122"`
	Desc      string     `mapstructure:"desc" json:"desc" validate:"uuid_rfc4122"`
	Delegates []Delegate `mapstructure:"delegates" json:"delegates" validate:"uuid_rfc4122"`
	Solutions []Solution `mapstructure:"solutions" json:"solutions" validate:"uuid_rfc4122"`
	Voters    []string   `mapstructure:"voters" json:"voters" validate:"uuid_rfc4122"`
}

type Solution struct {
	ID    string `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`
	Desc  string `mapstructure:"desc" json:"desc" validate:"uuid_rfc4122"`
	Votes int    `mapstructure:"votes" json:"votes" validate:"uuid_rfc4122"`
}

type Delegate struct {
	CitizenID string `mapstructure:"citizenId" json:"citizenId" validate:"uuid_rfc4122"`
	Votes     int    `mapstructure:"votes" json:"votes" validate:"uuid_rfc4122"`
	Selected  int    `mapstructure:"selected" json:"selected" validate:"uuid_rfc4122"`
	OwnVote   bool   `mapstructure:"voted" json:"voted" validate:"uuid_rfc4122"`
}

type CitizenReputation struct {
	ID              string  `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`
	Type            string  `mapstructure:"type" json:"type" validate:"uuid_rfc4122"`
	CitizenID       string  `mapstructure:"citizenId" json:"citizenId" validate:"uuid_rfc4122"`
	ReputationIndex float64 `mapstructure:"reputationIndex" json:"reputationIndex" validate:"uuid_rfc4122"`
}

func (a *acid) OnMount(ctx app.Context) {
	topic := "critical"
	a.topic = topic
	sh := shell.NewShell("localhost:5001")
	a.sh = sh
	myPeer, err := a.sh.ID()
	if err != nil {
		log.Fatal(err)
	}

	citizenID := myPeer.ID[len(myPeer.ID)-8:]
	// replace password with your own
	password := "mysecretpassword"

	a.citizenID = mixer.EncodeString(password, citizenID)
	a.citizenID = "3"
	a.subscribe(ctx)
	a.notifications = make(map[string]notification)

	ctx.Async(func() {
		// err := a.sh.OrbitDocsDelete(dbAddressIssue, "all")
		// if err != nil {
		// 	log.Fatal(err)
		// }

		// err := a.sh.OrbitDocsDelete(dbAddressCitizenReputation, "4")
		// if err != nil {
		// 	log.Fatal(err)
		// }

		cr, err := a.sh.OrbitDocsQuery(dbAddressCitizenReputation, "type", "reputation")
		if err != nil {
			log.Fatal(err)
		}

		var cc []interface{}
		err = json.Unmarshal(cr, &cc)
		if err != nil {
			log.Fatal(err)
		}

		for _, zz := range cc {
			r := CitizenReputation{}
			err = mapstructure.Decode(zz, &r)
			if err != nil {
				log.Fatal(err)
			}
			ctx.Dispatch(func(ctx app.Context) {
				a.ranks = append(a.ranks, r)
				sort.SliceStable(a.ranks, func(i, j int) bool {
					return a.ranks[i].ID < a.ranks[j].ID
				})
			})
		}

		v, err := a.sh.OrbitDocsQuery(dbAddressIssue, "type", "shortage")
		if err != nil {
			log.Fatal(err)
		}

		var vv []interface{}
		err = json.Unmarshal(v, &vv)
		if err != nil {
			log.Fatal(err)
		}

		for _, ii := range vv {
			i := Issue{}
			err = mapstructure.Decode(ii, &i)
			if err != nil {
				log.Fatal(err)
			}
			ctx.Dispatch(func(ctx app.Context) {
				a.issues = append(a.issues, i)
				sort.SliceStable(a.issues, func(i, j int) bool {
					return a.issues[i].ID > a.issues[j].ID
				})
			})
		}
	})
}

func (a *acid) subscribe(ctx app.Context) {
	ctx.Async(func() {
		subscription, err := a.sh.PubSubSubscribe(a.topic)
		if err != nil {
			log.Fatal(err)
		}
		a.sub = subscription
		a.subscription(ctx)
	})
}

func (a *acid) subscription(ctx app.Context) {
	ctx.Async(func() {
		defer a.sub.Cancel()
		// wait on pubsub
		res, err := a.sub.Next()
		if err != nil {
			log.Fatal(err)
		}
		// Decode the string data.
		str := string(res.Data)
		log.Println("Subscriber of topic: " + a.topic + " received message: " + str)
		ctx.Async(func() {
			a.subscribe(ctx)
		})
		ctx.Dispatch(func(ctx app.Context) {
			s := Issue{}
			s.Desc = str
			var lastID int
			unique := true
			for n, i := range a.issues {
				if s.Desc == i.Desc {
					unique = false
				}

				if n > 0 {
					currentID, err := strconv.Atoi(i.ID)
					if err != nil {
						log.Fatal(err)
					}
					previousID, err := strconv.Atoi(a.issues[n-1].ID)
					if err != nil {
						log.Fatal(err)
					}
					if currentID > previousID {
						lastID = currentID
					}
				}

			}
			if unique {
				newID := lastID + 1
				issue := Issue{
					ID:        strconv.Itoa(newID),
					Type:      typeShortage,
					Desc:      s.Desc,
					Solutions: []Solution{},
				}

				i, err := json.Marshal(issue)
				if err != nil {
					log.Fatal(err)
				}
				ctx.Async(func() {
					err = a.sh.OrbitDocsPut(dbAddressIssue, i)
					if err != nil {
						log.Fatal(err)
					}
					ctx.Dispatch(func(ctx app.Context) {
						a.issues = append(a.issues, issue)
					})
				})
			}

		})
	})
}

// The Render method is where the component appearance is defined. Here, a
// "pubsub World!" is displayed as a heading.
func (a *acid) Render() app.UI {
	return app.Div().Class("l-application").Role("presentation").Body(
		app.Link().Rel("stylesheet").Href("https://assets.ubuntu.com/v1/vanilla-framework-version-3.8.0.min.css"),
		app.Link().Rel("stylesheet").Href("https://use.fontawesome.com/releases/v6.2.0/css/all.css"),
		app.Link().Rel("stylesheet").Href("/app.css"),
		app.Header().Class("l-navigation is-collapsed").Body(
			app.Div().Class("l-navigation__drawer").Body(
				app.Div().Class("p-panel is-dark").Body(
					app.Div().Class("p-panel__header is-sticky").Body(
						app.A().Class("p-panel__logo").Href("#").Body(
							app.H5().Class("p-heading--2").Text("Cyber Acid"),
						),
					),
					app.Hr(),
					app.P().Class("p-heading--6").Body(
						app.Text("Liquid democracy politics simulator based on the automated data feed from the moneyless economy simulator "),
						app.A().Href("https://github.com/stateless-minds/cyber-stasis").Text("Cyber Stasis"),
					).Style("padding", "0 10%;"),
					app.Hr(),
					app.Div().Class("p-panel__content").Body(
						app.Div().Class("p-side-navigation--icons is-dark").ID("drawer-icons").Body(
							app.Nav().Aria("label", "Main"),
							app.Ul().Class("p-side-navigation__list").Body(
								app.Li().Class("p-side-navigation__item--title").Body(
									app.A().Class("p-side-navigation__link").Href("#").Body(
										app.I().Class("p-icon--help is-light p-side-navigation__icon"),
										app.Span().Class("p-side-navigation__label").Text("How to play"),
									).OnClick(a.openHowToDialog),
									app.A().Class("p-side-navigation__link").Href("#").Body(
										app.I().Class("p-icon--warning is-light p-side-navigation__icon"),
										app.Span().Class("p-side-navigation__label").Text("Shortages"),
									).Aria("current", "page"),
									app.A().Class("p-side-navigation__link").Href("#").Body(
										app.I().Class("p-icon--share is-light p-side-navigation__icon"),
										app.Span().Class("p-side-navigation__label").Text("Delegate rankings"),
									).OnClick(a.openRankingsDialog),
								),
							),
						),
					),
				),
			),
		),
		app.Main().Class("l-main").Body(
			app.Div().Class("p-panel").Body(
				app.If(len(a.notifications) > 0,
					app.Range(a.notifications).Map(func(s string) app.UI {
						return app.Div().Class("p-notification--" + a.notifications[s].status).Body(
							app.Div().Class("p-notification__content").Body(
								app.H5().Class("p-notification__title").Text(a.notifications[s].header),
								app.P().Class("p-notification__message").Text(a.notifications[s].message),
							),
						)
					}),
				),
				app.Div().Class("p-panel__header").Body(
					app.H4().Class("p-panel__title").Text("Open Issues"),
				),
				app.Div().Class("p-panel__content").Body(
					app.Div().Class("u-fixed-width").Body(
						app.Table().Aria("label", "Issues table").Class("p-main-table").Body(
							app.THead().Body(
								app.Tr().Body(
									app.Th().Body(
										app.Span().Class("status-icon is-blocked").Text("Issues"),
									),
									app.Th().Text("Actions"),
								),
							),
							app.If(len(a.issues) > 0, app.TBody().Body(
								app.Range(a.issues).Slice(func(i int) app.UI {
									return app.Tr().DataSet("id", i).Body(
										app.Td().DataSet("column", "issue").Body(
											app.Div().Text(a.issues[i].Desc),
										),
										app.Td().DataSet("column", "action").Body(
											app.Div().Body(
												app.Button().Class("u-no-margin--bottom").Text("List Solutions").Value(a.issues[i].ID).OnMouseOver(a.asidePreloadList).OnClick(a.asideOpenList),
												app.Button().Class("u-no-margin--bottom").Text("Suggest Solution").Value(a.issues[i].ID).OnMouseOver(a.asidePreloadCreate).OnClick(a.asideOpenCreate),
											),
										),
									)
								}),
							)),
						),
					),
					app.Div().Class("p-modal").ID("howto-modal").Style("display", "none").Body(
						app.Section().Class("p-modal__dialog").Role("dialog").Aria("modal", true).Aria("labelledby", "modal-title").Aria("describedby", "modal-description").Body(
							app.Header().Class("p-modal__header").Body(
								app.H2().Class("p-modal__title").ID("modal-title").Text("How to play"),
								app.Button().Class("p-modal__close").Aria("label", "Close active modal").Aria("controls", "modal").OnClick(a.closeHowToModal),
							),
							app.Div().Class("p-heading-icon--small").Body(
								app.Aside().Class("p-accordion").Body(
									app.Ul().Class("p-accordion__list").Body(
										app.Li().Class("p-accordion__group").Body(
											app.Div().Role("heading").Aria("level", "3").Class("p-accordion__heading").Body(
												app.Button().Type("button").Class("p-accordion__tab").ID("tab1").Aria("controls", "tab1-section").Aria("expanded", true).Text("What is Cyber Acid").Value("tab1-section").OnClick(a.toggleAccordion),
											),
											app.Section().Class("p-accordion__panel").ID("tab1-section").Aria("hidden", false).Aria("labelledby", "tab1").Body(
												app.P().Text("Cyber Acid is a political simulator based on the liquid democracy concept. It is designed as an integration module that works with Cyber Stasis - the moneyless economy simulator."),
											),
										),
										app.Li().Class("p-accordion__group").Body(
											app.Div().Role("heading").Aria("level", "3").Class("p-accordion__heading").Body(
												app.Button().Type("button").Class("p-accordion__tab").ID("tab2").Aria("controls", "tab2-section").Aria("expanded", true).Text("What is liquid democracy").Value("tab2-section").OnClick(a.toggleAccordion),
											),
											app.Section().Class("p-accordion__panel").ID("tab2-section").Aria("hidden", true).Aria("labelledby", "tab1").Body(
												app.P().Text("Liquid democracy meets the transparency and accountability of direct democracy with the easy of use of representative democracy. Vote directly for what you want and delegate one-time voting rights per topic."),
											),
										),
										app.Li().Class("p-accordion__group").Body(
											app.Div().Role("heading").Aria("level", "3").Class("p-accordion__heading").Body(
												app.Button().Type("button").Class("p-accordion__tab").ID("tab3").Aria("controls", "tab3-section").Aria("expanded", true).Text("How it works").Value("tab3-section").OnClick(a.toggleAccordion),
											),
											app.Section().Class("p-accordion__panel").ID("tab3-section").Aria("hidden", true).Aria("labelledby", "tab3").Body(
												app.P().Text("The simulator receives live data from Cyber Stasis about critical shortages of production and resources. The goal of all participants is to suggest solutions to those issues. For example - replacing a resource with another one, researching new technologies etc."),
											),
										),
										app.Li().Class("p-accordion__group").Body(
											app.Div().Role("heading").Aria("level", "3").Class("p-accordion__heading").Body(
												app.Button().Type("button").Class("p-accordion__tab").ID("tab4").Aria("controls", "tab4-section").Aria("expanded", true).Text("Features").Value("tab4-section").OnClick(a.toggleAccordion),
											),
											app.Section().Class("p-accordion__panel").ID("tab4-section").Aria("hidden", true).Aria("labelledby", "tab3").Body(
												app.Ul().Class("p-matrix").Body(
													app.Li().Class("p-matrix__item").Body(
														app.Div().Class("p-matrix__content").Body(
															app.H3().Class("p-matrix__title").Text("Check shortages"),
															app.Div().Class("p-matrix__desc").Body(
																app.P().Text("Review pressing issues."),
															),
														),
													),
													app.Li().Class("p-matrix__item").Body(
														app.Div().Class("p-matrix__content").Body(
															app.H3().Class("p-matrix__title").Text("Suggest a solution"),
															app.Div().Class("p-matrix__desc").Body(
																app.P().Text("Contribute with your expertise."),
															),
														),
													),
													app.Li().Class("p-matrix__item").Body(
														app.Div().Class("p-matrix__content").Body(
															app.H3().Class("p-matrix__title").Text("Vote for solutions"),
															app.Div().Class("p-matrix__desc").Body(
																app.P().Text("Vote for the best solution."),
															),
														),
													),
													app.Li().Class("p-matrix__item").Body(
														app.Div().Class("p-matrix__content").Body(
															app.H3().Class("p-matrix__title").Text("Delegate your vote"),
															app.Div().Class("p-matrix__desc").Body(
																app.P().Text("Not competent? Delegate your vote."),
															),
														),
													),
													app.Li().Class("p-matrix__item").Body(
														app.Div().Class("p-matrix__content").Body(
															app.H3().Class("p-matrix__title").Text("Infinite delegation"),
															app.Div().Class("p-matrix__desc").Body(
																app.P().Text("Delegation can be chained for maximum participation."),
															),
														),
													),
													app.Li().Class("p-matrix__item").Body(
														app.Div().Class("p-matrix__content").Body(
															app.H3().Class("p-matrix__title").Text("Cross delegation"),
															app.Div().Class("p-matrix__desc").Body(
																app.P().Text("Cross delegation is also supported."),
															),
														),
													),
												),
											),
										),
										app.Li().Class("p-accordion__group").Body(
											app.Div().Role("heading").Aria("level", "3").Class("p-accordion__heading").Body(
												app.Button().Type("button").Class("p-accordion__tab").ID("tab5").Aria("controls", "tab5-section").Aria("expanded", true).Text("Support us").Value("tab5-section").OnClick(a.toggleAccordion),
											),
											app.Section().Class("p-accordion__panel").ID("tab5-section").Aria("hidden", true).Aria("labelledby", "tab5").Body(
												app.A().Href("https://opencollective.com/stateless-minds-collective").Text("https://opencollective.com/stateless-minds-collective"),
											),
										),
										app.Li().Class("p-accordion__group").Body(
											app.Div().Role("heading").Aria("level", "3").Class("p-accordion__heading").Body(
												app.Button().Type("button").Class("p-accordion__tab").ID("tab6").Aria("controls", "tab6-section").Aria("expanded", true).Text("Terms of service").Value("tab6-section").OnClick(a.toggleAccordion),
											),
											app.Section().Class("p-accordion__panel").ID("tab6-section").Aria("hidden", true).Aria("labelledby", "tab6").Body(
												app.Div().Class("p-card").Body(
													app.H3().Text("Introduction"),
													app.P().Class("p-card__content").Text("Cyber Acid is a liquid democracy political simulator in the form of a fictional game based on real-time data from Cyber Stasis. By using the application you are implicitly agreeing to share your peer id with the IPFS public network."),
												),
												app.Div().Class("p-card").Body(
													app.H3().Text("Application Hosting"),
													app.P().Class("p-card__content").Text("Cyber Acid is a decentralized application and is hosted on a public peer to peer network. By using the application you agree to host it on the public IPFS network free of charge for as long as your usage is."),
												),
												app.Div().Class("p-card").Body(
													app.H3().Text("User-Generated Content"),
													app.P().Class("p-card__content").Text("All published content is user-generated, fictional and creators are not responsible for it."),
												),
											),
										),
										app.Li().Class("p-accordion__group").Body(
											app.Div().Role("heading").Aria("level", "3").Class("p-accordion__heading").Body(
												app.Button().Type("button").Class("p-accordion__tab").ID("tab7").Aria("controls", "tab7-section").Aria("expanded", true).Text("Privacy policy").Value("tab7-section").OnClick(a.toggleAccordion),
											),
											app.Section().Class("p-accordion__panel").ID("tab7-section").Aria("hidden", true).Aria("labelledby", "tab7").Body(
												app.Div().Class("p-card").Body(
													app.H3().Text("Personal data"),
													app.P().Class("p-card__content").Text("There is no personal information collected within Cyber Acid. We store a small portion of your peer ID encrypted as a non-unique identifier which is used for displaying the ranks interface."),
												),
												app.Div().Class("p-card").Body(
													app.H3().Text("Coookies"),
													app.P().Class("p-card__content").Text("Cyber Acid does not use cookies."),
												),
												app.Div().Class("p-card").Body(
													app.H3().Text("Links to Cyber Stasis"),
													app.P().Class("p-card__content").Text("Cyber Acid contains links to its sister project Cyber Stasis and depends on its data to function properly."),
												),
												app.Div().Class("p-card").Body(
													app.H3().Text("Changes to this privacy policy"),
													app.P().Class("p-card__content").Text("This Privacy Policy might be updated from time to time. Thus, it is advised to review this page periodically for any changes. You will be notified of any changes from this page. Changes are effective immediately after they are posted on this page."),
												),
											),
										),
									),
								),
							),
						).Style("left", "10%").Style("width", "80%"),
					),
					app.Div().Class("p-modal").ID("rankings-modal").Style("display", "none").Body(
						app.Section().Class("p-modal__dialog").Role("dialog").Aria("modal", true).Aria("labelledby", "modal-title").Aria("describedby", "modal-description").Body(
							app.Header().Class("p-modal__header").Body(
								app.H2().Class("p-modal__title").ID("modal-title").Text("Delegate rankings"),
								app.Button().Class("p-modal__close").Aria("label", "Close active modal").Aria("controls", "modal").OnClick(a.closeRankingsModal),
							),
							app.Table().Aria("label", "Rankings table").Class("p-main-table").Body(
								app.THead().Body(
									app.Tr().Body(
										app.Th().Body(
											app.Span().Class("status-icon is-blocked").Text("Delegates"),
										),
										app.Th().Text("Trust"),
									),
								),
								app.If(len(a.delegates) > 0, app.TBody().Body(
									app.Range(a.delegates).Slice(func(i int) app.UI {
										return app.Tr().DataSet("id", i).Body(
											app.Td().DataSet("column", "delegate").Body(
												app.Div().Text(a.delegates[i].CitizenID),
											),
											app.Td().DataSet("column", "trust").Body(
												app.Div().Text(a.delegates[i].Selected),
											),
										)
									}),
								)),
							),
						).Style("left", "10%").Style("width", "80%"),
					),
				),
			),
		),
		app.Aside().Class("l-aside is-collapsed").ID("aside-panel").Body(
			app.Div().Class("p-panel").Body(
				app.Div().Class("p-panel__header").Body(
					app.H4().Class("p-panel__title").Text(a.AsideTitle),
					app.Div().Class("p-panel__controls").Body(
						app.Button().Class("p-button--base u-no-margin--bottom has-icon").Body(app.I().Class("p-icon--close")).OnClick(a.asideClose),
					),
				),
				app.If(a.AsideTitle == asideTitleCreate,
					app.Div().Class("p-panel__content").Body(
						app.Div().Class("p-form p-form--stacked").Body(
							app.Div().Class("p-form__group row").Body(
								app.Textarea().ID("solution").Name("solution").Rows(3).OnKeyUp(a.onSolution),
							),
						),
						app.Div().Class("row").Body(
							app.Div().Class("col-12").Body(
								app.Button().Class("p-button--positive u-float-right").Name("submit-solution").Text("Submit Solution").OnClick(a.submitSolution),
							),
						),
					),
				).ElseIf(a.AsideTitle == asideTitleList,
					app.Div().Class("p-panel__content").Body(
						app.Ul().Class("p-list-tree").Aria("multiselectable", true).Role("tree").Body(
							app.Li().Class("p-list-tree__item p-list-tree__item--group").Role("treeitem").Body(
								app.Button().Class("p-list-tree__toggle").ID("sub-1-btn").Aria("controls", "sub-1").Aria("expanded", true).Text("Suggested Solutions"),
								app.Ul().Class("p-list-tree").Role("group").ID("sub-1").Aria("hidden", false).Aria("labelledby", "sub-1-btn").Body(
									app.Range(a.Solutions).Slice(func(i int) app.UI {
										return app.Li().Class("p-list-tree__item").Role("treeitem").Body(
											app.P().Text(a.Solutions[i].Desc),
											app.If(len(a.issues[a.currentIssueInSlice].Voters) > 0,
												app.If(sliceContains(a.issues[a.currentIssueInSlice].Voters, a.citizenID),
													app.Button().Class("p-button is-small is-inline").Text("Vote").Value(a.Solutions[i].ID).OnClick(a.vote).Disabled(true).Body(
														app.I().Class("fa-solid fa-thumbs-up"),
													),
													app.Span().Class("p-badge").Aria("label", strconv.Itoa(a.Solutions[i].Votes)+"votes").Text(a.Solutions[i].Votes),
												).Else(
													app.Button().Class("p-button is-small is-inline").Text("Vote").Value(a.Solutions[i].ID).OnClick(a.vote).Body(
														app.I().Class("fa-regular fa-thumbs-up"),
													),
													app.Button().Class("p-button is-small is-inline").ID("show-modal").Text("Delegate...").Aria("controls", "modal").Value(a.Solutions[i].ID).OnClick(a.openDelegateDialog),
													app.Div().Class("p-modal").ID("delegate-modal").Style("display", "none").Body(
														app.Section().Class("p-modal__dialog").Role("dialog").Aria("modal", true).Aria("labelledby", "modal-title").Aria("describedby", "modal-description").Body(
															app.Header().Class("p-modal__header").Body(
																app.H2().Class("p-modal__title").ID("modal-title").Text("Delegate"),
																app.Button().Class("p-modal__close").Aria("label", "Close active modal").Aria("controls", "modal").Value(a.Solutions[i].ID).OnClick(a.closeDelegateModal),
															),
															app.P().ID("modal-description").Text("Select a citizen to represent your vote for this issue:"),
															app.Div().Class("p-heading-icon--small").Body(
																app.Range(a.ranks).Slice(func(i int) app.UI {
																	return app.If(a.ranks[i].CitizenID != a.citizenID,
																		app.Div().Class("p-heading-icon__header").Body(
																			app.Button().Class("p-chip").Aria("pressed", true).Disabled(true).Body(
																				app.Span().Class("p-chip__value").Text("Citizen"),
																				app.Span().Class("p-badge").Aria("label", "Citizen").Text(a.ranks[i].CitizenID),
																			),
																			app.Button().Class("p-chip").Aria("pressed", true).Disabled(true).Body(
																				app.Span().Class("p-chip__value").Text("Reputation"),
																				app.Span().Class("p-badge").Aria("label", "Reputation").Text(a.ranks[i].ReputationIndex),
																			),
																			app.Button().Class("p-chip").Body(
																				app.Span().Class("p-chip__value").Text("Select"),
																			).Value(a.ranks[i].CitizenID).OnClick(a.delegate),
																		),
																	)
																}),
															),
														),
													),
												),
											).Else(
												app.Button().Class("p-button is-small is-inline").Text("Vote").Value(a.Solutions[i].ID).OnClick(a.vote).Body(
													app.I().Class("fa-regular fa-thumbs-up"),
												),
												app.Button().Class("p-button is-small is-inline").ID("show-modal").Text("Delegate...").Aria("controls", "modal").Value(a.Solutions[i].ID).OnClick(a.openDelegateDialog),
												app.Div().Class("p-modal").ID("delegate-modal").Style("display", "none").Body(
													app.Section().Class("p-modal__dialog").Role("dialog").Aria("modal", true).Aria("labelledby", "modal-title").Aria("describedby", "modal-description").Body(
														app.Header().Class("p-modal__header").Body(
															app.H2().Class("p-modal__title").ID("modal-title").Text("Delegate"),
															app.Button().Class("p-modal__close").Aria("label", "Close active modal").Aria("controls", "modal").Value(a.Solutions[i].ID).OnClick(a.closeDelegateModal),
														),
														app.P().ID("modal-description").Text("Select a citizen to represent your vote for this issue:"),
														app.Div().Class("p-heading-icon--small").Body(
															app.Range(a.ranks).Slice(func(i int) app.UI {
																return app.If(a.ranks[i].CitizenID != a.citizenID,
																	app.Div().Class("p-heading-icon__header").Body(
																		app.Button().Class("p-chip").Aria("pressed", true).Disabled(true).Body(
																			app.Span().Class("p-chip__value").Text("Citizen"),
																			app.Span().Class("p-badge").Aria("label", "Citizen").Text(a.ranks[i].CitizenID),
																		),
																		app.Button().Class("p-chip").Aria("pressed", true).Disabled(true).Body(
																			app.Span().Class("p-chip__value").Text("Reputation"),
																			app.Span().Class("p-badge").Aria("label", "Reputation").Text(a.ranks[i].ReputationIndex),
																		),
																		app.Button().Class("p-chip").Body(
																			app.Span().Class("p-chip__value").Text("Select"),
																		).Value(a.ranks[i].CitizenID).OnClick(a.delegate),
																	),
																)
															}),
														),
													),
												),
											),
										)
									}),
								),
							),
						),
					),
				),
			),
		),
	)
}

func (a *acid) asidePreloadList(ctx app.Context, e app.Event) {
	issueID := ctx.JSSrc().Get("value").String()
	issueIDInt, err := strconv.Atoi(issueID)
	if err != nil {
		log.Fatal(err)
	}
	a.currentIssueInSlice = issueIDInt - 1
	a.Solutions = a.issues[issueIDInt-1].Solutions
	a.AsideTitle = asideTitleList
}

func (a *acid) asideOpenList(ctx app.Context, e app.Event) {
	app.Window().Get("document").Call("querySelector", ".l-aside").Get("classList").Call("remove", "is-collapsed")
}

func (a *acid) asidePreloadCreate(ctx app.Context, e app.Event) {
	a.AsideTitle = asideTitleCreate
}

func (a *acid) asideOpenCreate(ctx app.Context, e app.Event) {
	app.Window().Get("document").Call("querySelector", ".l-aside").Get("classList").Call("remove", "is-collapsed")
	app.Window().Get("document").Call("querySelector", ".p-button--positive").Call("setAttribute", "id", ctx.JSSrc().Get("value").String())
}

func (a *acid) asideClose(ctx app.Context, e app.Event) {
	app.Window().Get("document").Call("querySelector", ".l-aside").Get("classList").Call("add", "is-collapsed")
}

func (a *acid) onSolution(ctx app.Context, e app.Event) {
	a.currentSolutionDesc = ctx.JSSrc().Get("value").String()
}

func (a *acid) vote(ctx app.Context, e app.Event) {
	ctx.JSSrc().Get("firstChild").Get("classList").Call("remove", "fa-regular")

	val := ctx.JSSrc().Get("value").String()
	solutionID, err := strconv.Atoi(val)
	if err != nil {
		log.Fatal(err)
	}

	currentIssue := a.issues[a.currentIssueInSlice]
	var delegate bool
	var delegatedVotes int
	var ownVote bool
	// delegated voting logic
	for i, d := range currentIssue.Delegates {
		if a.citizenID == d.CitizenID {
			delegate = true
			ownVote = currentIssue.Delegates[i].OwnVote
			if !d.OwnVote {
				currentIssue.Delegates[i].OwnVote = true

			}
			delegatedVotes = d.Votes
			currentIssue.Delegates[i].Votes = 0
		}
	}

	currentIssue.Voters = append(currentIssue.Voters, a.citizenID)

	if delegate {
		if !ownVote {
			currentIssue.Solutions[solutionID-1].Votes += delegatedVotes + 1
		} else {
			currentIssue.Solutions[solutionID-1].Votes += delegatedVotes
		}

	} else {
		currentIssue.Solutions[solutionID-1].Votes++
	}

	i, err := json.Marshal(currentIssue)
	if err != nil {
		log.Fatal(err)
	}
	ctx.Async(func() {
		err = a.sh.OrbitDocsPut(dbAddressIssue, i)
		if err != nil {
			ctx.Dispatch(func(ctx app.Context) {
				a.createNotification(ctx, NotificationDanger, ErrorHeader, "Could not vote for solution. Try again later.")
				log.Fatal(err)
			})
		}
		ctx.Dispatch(func(ctx app.Context) {
			a.issues[a.currentIssueInSlice] = currentIssue
			a.createNotification(ctx, NotificationSuccess, SuccessHeader, "Vote accepted.")
		})
	})
}

func (a *acid) openRankingsDialog(ctx app.Context, e app.Event) {
	for _, i := range a.issues {
		a.delegates = append(a.delegates, i.Delegates...)
	}
	sort.SliceStable(a.delegates, func(i, j int) bool {
		return a.delegates[i].Selected > a.delegates[j].Selected
	})
	fmt.Println(a.delegates)
	app.Window().GetElementByID("rankings-modal").Set("style", "display:flex")
}

func (a *acid) openHowToDialog(ctx app.Context, e app.Event) {
	app.Window().GetElementByID("howto-modal").Set("style", "display:flex")
}

func (a *acid) openDelegateDialog(ctx app.Context, e app.Event) {
	app.Window().GetElementByID("delegate-modal").Set("style", "display:flex")
	solutionID := ctx.JSSrc().Get("value").String()
	solutionIDInt, err := strconv.Atoi(solutionID)
	if err != nil {
		log.Fatal(err)
	}
	a.currentSolutionInSlice = solutionIDInt - 1
}

func (a *acid) delegate(ctx app.Context, e app.Event) {
	citizenID := ctx.JSSrc().Get("value").String()
	issue := a.issues[a.currentIssueInSlice]

	var delegateExists bool
	var delegate Delegate
	votesTransfer := 1
	if len(issue.Delegates) > 0 {
		for ii, dd := range issue.Delegates {
			// recursive delegation logic
			if dd.CitizenID == a.citizenID {
				// transfer origin votes to recipient plus own vote
				votesTransfer = dd.Votes + 1
				// set origin delegator's votes to zero
				issue.Delegates[ii].Votes = 0
				// set origin delegator as voted
				issue.Delegates[ii].OwnVote = true
			}
		}

		for i, d := range issue.Delegates {
			if d.CitizenID == citizenID {
				issue.Delegates[i].Votes += votesTransfer
				issue.Delegates[i].Selected++
				delegateExists = true
			}
		}
	}

	if len(issue.Delegates) == 0 || !delegateExists {
		delegate = Delegate{
			CitizenID: citizenID,
			Votes:     votesTransfer,
			Selected:  1,
		}
		issue.Delegates = append(issue.Delegates, delegate)
	}
	issue.Voters = append(issue.Voters, a.citizenID)
	var voters []string
	for _, v := range issue.Voters {
		// if the delegate already voted previously remove from voters so he can vote again on new delegation
		if citizenID != v {
			voters = append(voters, v)
		}
	}
	issue.Voters = voters

	i, err := json.Marshal(issue)
	if err != nil {
		log.Fatal(err)
	}

	ctx.Async(func() {
		err = a.sh.OrbitDocsPut(dbAddressIssue, i)
		if err != nil {
			log.Fatal(err)
		}

		ctx.Dispatch(func(ctx app.Context) {
			a.issues[a.currentIssueInSlice] = issue
			a.closeDelegateModal(ctx, e)
			a.createNotification(ctx, NotificationSuccess, SuccessHeader, "Vote delegated.")
		})
	})
}

func (a *acid) toggleAccordion(ctx app.Context, e app.Event) {
	id := ctx.JSSrc().Get("value").String()
	attr := app.Window().GetElementByID(id).Get("attributes")
	aria := attr.Get("aria-hidden").Get("value").String()
	if aria == "false" {
		app.Window().GetElementByID(id).Call("setAttribute", "aria-hidden", "true")
	} else {
		app.Window().GetElementByID(id).Call("setAttribute", "aria-hidden", "false")
	}
}

func (a *acid) closeRankingsModal(ctx app.Context, e app.Event) {
	app.Window().GetElementByID("rankings-modal").Set("style", "display:none")
}

func (a *acid) closeHowToModal(ctx app.Context, e app.Event) {
	app.Window().GetElementByID("howto-modal").Set("style", "display:none")
}

func (a *acid) closeDelegateModal(ctx app.Context, e app.Event) {
	app.Window().GetElementByID("delegate-modal").Set("style", "display:none")
}

func (a *acid) submitSolution(ctx app.Context, e app.Event) {
	idStr := ctx.JSSrc().Get("id").String()
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Fatal(err)
	}

	lastSolutionID := 0
	unique := true
	if len(a.issues[id-1].Solutions) > 0 {
		solutions := a.issues[id-1].Solutions
		for n, s := range solutions {
			if s.Desc == a.currentSolutionDesc {
				unique = false
			}
			if n > 0 {
				currentID, err := strconv.Atoi(s.ID)
				if err != nil {
					log.Fatal(err)
				}
				previousID, err := strconv.Atoi(solutions[n-1].ID)
				if err != nil {
					log.Fatal(err)
				}
				if currentID > previousID {
					lastSolutionID = currentID
				}
			} else {
				lastSolutionID = 1
			}
		}
	}

	if unique {
		solution := Solution{
			ID:    strconv.Itoa(lastSolutionID + 1),
			Desc:  a.currentSolutionDesc,
			Votes: 0,
		}

		a.issues[id-1].Solutions = append(a.issues[id-1].Solutions, solution)

		i, err := json.Marshal(a.issues[id-1])
		if err != nil {
			log.Fatal(err)
		}

		ctx.Async(func() {
			err = a.sh.OrbitDocsPut(dbAddressIssue, i)
			if err != nil {
				ctx.Dispatch(func(ctx app.Context) {
					a.createNotification(ctx, NotificationDanger, ErrorHeader, "Could not create solution. Try again later.")
					log.Fatal(err)
				})
			}
			ctx.Dispatch(func(ctx app.Context) {
				app.Window().Get("document").Call("querySelector", ".l-aside").Get("classList").Call("add", "is-collapsed")
				a.createNotification(ctx, NotificationSuccess, SuccessHeader, "Solution submited.")
			})
		})
	}
}

func (a *acid) createNotification(ctx app.Context, s NotificationStatus, h string, msg string) {
	a.notificationID++
	a.notifications[strconv.Itoa(a.notificationID)] = notification{
		id:      a.notificationID,
		status:  string(s),
		header:  h,
		message: msg,
	}

	ntfs := a.notifications
	ctx.Async(func() {
		for n := range ntfs {
			time.Sleep(5 * time.Second)
			delete(ntfs, n)
			ctx.Async(func() {
				ctx.Dispatch(func(ctx app.Context) {
					a.notifications = ntfs
				})
			})
		}
	})
}

// https://play.golang.org/p/Qg_uv_inCek
// contains checks if a string is present in a slice
func sliceContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
