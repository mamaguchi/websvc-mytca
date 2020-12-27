package my_ldap

import (
	"log"
	"fmt"
	"github.com/go-ldap/ldap/v3"
	// "github.com/google/uuid"
	"strings"
	// "strconv"
	"encoding/json"
	"net/http"
)

type PkdClinics struct {
	PkdAdmId string         `json:"pkdAdmId"`
	JknName string          `json:"jknName"`
	PkdName string          `json:"pkdName"`
	ClinicIds []string       `json:"clinicIds"`
	ClinicNames []string    `json:"clinicNames"`
}

func GetPkdClinics(userId string, userPwd string) (output []byte, err error) {
    l, err := ldap.DialURL("ldap://127.0.0.1:389")
    if err != nil {
        log.Print(err)
		return
    }
    defer l.Close()

    userDN := fmt.Sprintf(STAFF_DN, userId)

    err = l.Bind(userDN, userPwd)
    if err != nil {
	    log.Print(err)
		return
    }

    searchFilter := fmt.Sprintf("(staffId=%s)", userId)
    searchReq := ldap.NewSearchRequest(
        USER_BASE_DN,
        ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0,0, false,
        searchFilter,
        []string{"groupUid"},
        nil,
    )
    sr, err := l.Search(searchReq)
    if err != nil {
		log.Print(err)
		return
    }

    groupUid := sr.Entries[0].GetAttributeValues("groupUid")
    groupUidSegs := strings.Split(groupUid[0], "-")
    pkdName := groupUidSegs[0]
    jknName := groupUidSegs[1]

    clinicSearchDN := fmt.Sprintf("ou=%s,ou=%s,ou=kkm-clinic,ou=groups,dc=example,dc=com",
                                pkdName, jknName)
    searchReq = ldap.NewSearchRequest(
            clinicSearchDN,
            ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0,0, false,
            "(&)",
            []string{"clinicName", "cn"},
            nil,
    )           
    sr, err = l.Search(searchReq)                     
    if err != nil {
        log.Print(err)
		return
	}
	
    var clinicNames, clinicIds []string
    for _, entry := range sr.Entries {
        clinicNames = append(clinicNames, entry.GetAttributeValue("clinicName"))
        clinicIds = append(clinicIds, entry.GetAttributeValue("cn"))
    }

    pkdClinics := PkdClinics{
            PkdAdmId: userId,
            JknName: jknName,
            PkdName: pkdName,
            ClinicIds: clinicIds,
            ClinicNames: clinicNames,
    }
	fmt.Printf("PkdClinics: %+v \n", pkdClinics)

    pkdClinicsJson, err := json.MarshalIndent(pkdClinics, "", "\t")
    if err != nil {
        log.Print(err)
		return
    }
    return pkdClinicsJson, err
}

func GetPkdClinicsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Headers", "authorization")	
	r.ParseForm()

	fmt.Println("[GetPkdClinicsHandler] Request Form Data Received!\n")
	fmt.Println(r.Form)
	
	userId := r.Form["userId"][0]
	userPwd := r.Form["userPwd"][0]		
        
	pkdClinics, err := GetPkdClinics(userId, userPwd)
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Fprintf(w, "%s", pkdClinics)
}