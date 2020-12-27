package my_ldap

import (
	"log"
	"fmt"
	"strconv"
	"github.com/go-ldap/ldap/v3"
	"encoding/json"
	"net/http"
	// "strings"
)

type ClinicServiceMeta struct {
	NumOfStaff int 				`json:"numOfStaff"`
	AvaiDays string				`json:"avaiDays"'`
	StartOpHrs string 			`json:"startOpHrs"`
	EndOpHrs string				`json:"endOpHrs"`
}


type AllClinics struct {
	States []State 			`json:"states"`
}

type State struct {
	Name string				`json:"name"`
	Districts []District 	`json:"districts"`
}

type District struct {
	Name string				`json:"name"`
	Clinics []Clinic 		`json:"clinics"`
}

type Clinic struct {
	Name string 			`json:"name"`
	Id string				`json:"id"`
}

// States
var johor = State{Name: "Johor"}
var kedah = State{Name: "Kedah"}
var kelantan = State{Name: "Kelantan"}
var kualaLumpur = State{Name: "Kuala Lumpur"}
var melaka = State{Name: "Melaka"}
var negeriSembilan = State{Name: "Negeri Sembilan"}
var pahang = State{Name: "Pahang"}
var perak = State{Name: "Perak"}
var perlis = State{Name: "Perlis"}
var pulauPinang = State{Name: "Pulau Pinang"}
var pulauLabuan = State{Name: "Pulau Labuan"}
var sabah = State{Name: "Sabah"}
var sarawak = State{Name: "Sarawak"}
var selangor = State{Name: "Selangor"}
var terengganu = State{Name: "Terengganu"}
// Districts
var maran = District{Name: "Maran"}
var termeloh = District{Name: "Termeloh"}
var klang = District{Name: "Klang"}

// Struct Containing List of All Clinics in Malaysia.
var ALL_CLINICS AllClinics

func init() {
	pahang.Districts = []District{maran, termeloh}
	selangor.Districts = []District{klang}

	states := []State{
					johor, 
					kedah, 
					kelantan,						
					kualaLumpur,
					melaka,  
					negeriSembilan, 
					pahang, 
					perak, 
					perlis, 
					pulauPinang, 
					pulauLabuan,
					sabah, 
					sarawak, 
					selangor,						
					terengganu, 
					}

	ALL_CLINICS.States = states
}


// EXPORTED FUNC
// =============
type DeptNameAndStaffNumStruct struct {
	Name string
	NumOfStaff int
}

// Get the name and number of staff of a clinic
func GetDeptNameAndStaffNum(userId string, userPwd string, clinicId string, 
				district string, state string) (output []DeptNameAndStaffNumStruct, err error) {
	
	l, err := ldap.DialURL("ldap://127.0.0.1:389")
    if err != nil {
        log.Print(err)
		return 
    }
	defer l.Close()
	
	// TODO: KIV to change 'staffId' to 'userId' by changing
	//       the LDAP object class from 'staff' to regular 'user'
	userDN := fmt.Sprintf(STAFF_DN, userId)
	err = l.Bind(userDN, userPwd)
	if err != nil {
		log.Print(err)
		return 
	}

	deptBaseDN := fmt.Sprintf(DEPT_BASE_DN, clinicId, district, state)
	searchFilter := "(clinicDeptName=*)"							
	searchReq := ldap.NewSearchRequest(
		deptBaseDN,
		ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0,0, false,
		searchFilter,
		[]string{"clinicDeptName", "clinicDeptNumOfStaff"},
		nil,
	)
	sr, err := l.Search(searchReq)                     
	if err != nil {
		log.Print(err)
		return 
	}

	var deptDataList []DeptNameAndStaffNumStruct
	for _, entry := range sr.Entries {
		deptName := entry.GetAttributeValue("clinicDeptName")
		deptNumOfStaff, _ := strconv.Atoi(entry.GetAttributeValue("clinicDeptNumOfStaff"))
		
		deptData := DeptNameAndStaffNumStruct{
			Name: deptName,
			NumOfStaff: deptNumOfStaff,			
		}
		deptDataList = append(deptDataList, deptData)
	}
	return deptDataList, nil
}

// Create a list of all the clinics in every district
// from every states in Malaysia.
func GetAllClinics() (output []byte, err error) {

	l, err := ldap.DialURL("ldap://127.0.0.1:389")
    if err != nil {
        log.Print(err)
		return 
    }
	defer l.Close()
		
	err = l.Bind(DIR_MGR_DN, DIR_MGR_PWD)
	if err != nil {
		log.Print(err)
		return 
	}

	acCopy := ALL_CLINICS
	for i, state := range acCopy.States {
		stateName := state.Name

		for j, district := range state.Districts {
			districtName := district.Name

			clinicSearchDN := fmt.Sprintf(CLINIC_BASE_DN, districtName, stateName)
			searchReq := ldap.NewSearchRequest(
				clinicSearchDN,
				ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0,0, false,
				"(&)",
				[]string{"clinicName", "cn"},
				nil,
			)
			sr, err := l.Search(searchReq)                     
			if err != nil {
				log.Print(err)
				return nil, err
			}

			for _, entry := range sr.Entries {
				clinicName := entry.GetAttributeValue("clinicName")
				clinicId := entry.GetAttributeValue("cn")
				clinic := Clinic{Name: clinicName, Id: clinicId}

				acCopy.States[i].Districts[j].Clinics = append(acCopy.States[i].Districts[j].Clinics, clinic)
			}
		}
	}	 
	output, err = json.MarshalIndent(acCopy, "", "\t")
	return 
}

func GetAllClinicsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")		
	w.Header().Set("Access-Control-Allow-Headers", "authorization")	

	fmt.Println("[GetAllClinicsHandler] Request Received!\n")

	allClinicsJson, err := GetAllClinics()
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Fprintf(w, "%s", allClinicsJson)
}

type SvcsCheckedList struct {
	SvcsChecked []SvcChecked	`json:"svcsChecked"`
}

type SvcChecked struct {
	Name string					`json:"svcName"`
	IfExist bool				`json:"ifExist"`
}

// Check if the service is provided by the clinic.
func CheckIfSvcExist(svcs []string, clinicId string, district string, 
					state string) (scl SvcsCheckedList, err error) {

	l, err := ldap.DialURL("ldap://127.0.0.1:389")
    if err != nil {
        log.Print(err)
		return 
    }
    defer l.Close()

	err = l.Bind(DIR_MGR_DN, DIR_MGR_PWD)
    if err != nil {
		log.Print(err)
		return 
	}

	for _, svc := range svcs {
		var ifSvcExist bool
		dept := SvcToDeptMap[svc] 

		svcDN := fmt.Sprintf(SERVICE_DN, svc, dept, clinicId, district, state)
		searchReq := ldap.NewSearchRequest(
			svcDN,
			ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0,0, false,
			"(&)",
			[]string{"clinicServiceName", "clinicServiceIsEnabled"}, 
			nil,
		)
		sr, err := l.Search(searchReq)                     
		if err != nil {
			// If the entry is not found in LDAP directory, the 
			// search request will return the following error:
			// """
			// LDAP Result Code 32 "No Such Object"
			// """
			// So we handle the error here accordingly.
			ifSvcExist = false 
			err = nil 
			continue
		}

		entry := sr.Entries[0]
		if entry.GetAttributeValue("clinicServiceName") == svc && 
		entry.GetAttributeValue("clinicServiceIsEnabled") == "1" {
			ifSvcExist = true
		} else {
			ifSvcExist = false
		}

		svcChecked := SvcChecked{Name: svc, IfExist: ifSvcExist}
		scl.SvcsChecked = append(scl.SvcsChecked, svcChecked)
	}	
	fmt.Printf("SvcsCheckedList: %+v \n", scl)
	return scl, nil 
}

func CheckIfSvcExistHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "authorization")	
	r.ParseForm()

	//TODO: This line is an ad-hoc solution to remove the 
	//      std output triggered by CORS preflighted request via http OPTIONS.
	//      The correct solution is to add a reverse-proxy server to the
	//      main server so that all requests are channeled to this reverse proxy
	//      in order to overcome CORS restriction.
	if (r.Method == "OPTIONS") { return }

	fmt.Println("[checkIfSvcExistHandler] Request Form Data Received!\n")
	fmt.Println(r.PostForm)
	
	clinicId := r.PostForm["clinicId"][0]
	district := r.PostForm["district"][0]
	state := r.PostForm["state"][0]
	
	svcsToCheck := []string{
		SVC_FUNDOSCOPY,
		SVC_XRAY,
	}
	scl, err := CheckIfSvcExist(svcsToCheck, clinicId, district, state)
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	
	sclJson, err := json.MarshalIndent(scl, "", "\t")
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Fprintf(w, "%s", sclJson)
}

type SvcOpHrs struct {
	ClinicPublicHoliday []string	`json:"clinicPublicHoliday"`
	AvaiDays string 				`json:"avaiDays"`
	StartHrs string					`json:"startHrs"` 
	EndHrs string					`json:"endHrs"`
}

// Get the operating start hour and end hour for
// the specified clinic service. 
func GetSvcOpHrs(svc string, dept string, clinicId string, district string, 
				state string) (svcOpHrs SvcOpHrs, err error) {

	l, err := ldap.DialURL("ldap://127.0.0.1:389")
    if err != nil {
        log.Print(err)
		return 
    }
	defer l.Close()
	
	// TODO: Use other less priviledged user
	//       account to bind to LDAP.
	err = l.Bind(DIR_MGR_DN, DIR_MGR_PWD)

	if err != nil {
		log.Print(err)
		return 
	}

	// Find public holidays
	clinicDN := fmt.Sprintf(CLINIC_DN, clinicId, district, state)
	searchReq := ldap.NewSearchRequest(
		clinicDN,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0,0, false,
		"(&)",
		[]string{"publicHolByMonth"},
		nil,
	)
	sr, err := l.Search(searchReq)
	if err != nil {
		log.Print(err)
		return 
	}
	svcOpHrs.ClinicPublicHoliday = sr.Entries[0].GetAttributeValues("publicHolByMonth")

	// Find service available days, start hour ,end hour.
	svcDN := fmt.Sprintf(SERVICE_DN, svc, dept, clinicId, district, state)
	searchReq = ldap.NewSearchRequest(
		svcDN,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0,0, false,
		"(&)",
		[]string{"clinicServiceStartHour", "clinicServiceEndHour", "clinicServiceAvaiDays"}, 
		nil,
	)
	sr, err = l.Search(searchReq)                     
	if err != nil {
		log.Print(err)
		return 
	}

	svcOpHrs.AvaiDays = sr.Entries[0].GetAttributeValue("clinicServiceAvaiDays")
	svcOpHrs.StartHrs = sr.Entries[0].GetAttributeValue("clinicServiceStartHour")
	svcOpHrs.EndHrs = sr.Entries[0].GetAttributeValue("clinicServiceEndHour")	
	fmt.Printf("svcOpHrs: %+v \n", svcOpHrs)
	return 
}

func GetSvcOpHrsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")		
	w.Header().Set("Access-Control-Allow-Headers", "authorization")	
	r.ParseForm()

	//TODO: This line is an ad-hoc solution to remove the 
	//      std output triggered by CORS preflighted request via http OPTIONS.
	//      The correct solution is to add a reverse-proxy server to the
	//      main server so that all requests are channeled to this reverse proxy
	//      in order to overcome CORS restriction.
	if (r.Method == "OPTIONS") { return }

	fmt.Println("[GetSvcOpHrsHandler] Request Form Data Received!\n")
	fmt.Println(r.PostForm)

	svc := r.PostForm["service"][0]
	dept := SvcToDeptMap[svc]
	clinicId := r.PostForm["clinicId"][0]
	district := r.PostForm["district"][0]
	state := r.PostForm["state"][0]

	svcOpHrs, err := GetSvcOpHrs(svc, dept, clinicId, district, state)
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}

	svcOpHrsJson, err := json.MarshalIndent(&svcOpHrs, "", "\t")
    if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Fprintf(w, "%s", svcOpHrsJson)
}





// Get the clinic service metadata via OpenDJ LDAP.
// func GetClinicServiceMeta(service string, clinic string, district string, state string) (serviceMeta ClinicServiceMeta, err error) {
// 	l, err := ldap.DialURL("ldap://127.0.0.1:389")
//     if err != nil {
//             log.Fatal(err)
//     }
//     defer l.Close()

// 	err = l.Bind(DIR_MGR_DN, DIR_MGR_PWD)
//     if err != nil {
// 		return 
// 	}

// 	serviceSearchDN := fmt.Sprintf(SERVICE_TEMPLATE_DN,
// 							service, clinic, district, state)
// 	searchRequest := ldap.NewSearchRequest(
// 		serviceSearchDN,
// 		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0,0, false,
// 		"(&)",
// 		[]string{"clinicServiceNumOfStaff",
// 				"clinicServiceAvaiDays", 
// 				"clinicServiceStartHour",
// 				"clinicServiceEndHour"},
// 		nil,
// 	)
// 	sr, err := l.Search(searchRequest)                     
// 	if err != nil {
// 			log.Fatal(err)
// 	}

// 	for _, entry := range sr.Entries {
// 		serviceMeta.NumOfStaff, _ = strconv.Atoi(entry.GetAttributeValue("clinicServiceNumOfStaff"))
// 		serviceMeta.AvaiDays = entry.GetAttributeValue("clinicServiceAvaiDays")
// 		serviceMeta.StartOpHrs = entry.GetAttributeValue("clinicServiceStartHour")
// 		serviceMeta.EndOpHrs = entry.GetAttributeValue("clinicServiceEndHour")
// 	}

// 	return serviceMeta, nil
// }







