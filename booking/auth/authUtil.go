package auth

import (
	"log"
	"fmt"
	"net/http"
	"time"
	"encoding/json"
	"github.com/go-ldap/ldap/v3"
	"github.com/dgrijalva/jwt-go"
	"mytca/booking/db/my_ldap"
)

// For HMAC signing method, the secret key can be any []byte. It is recommended to generate
// a key using crypto/rand or something equivalent. You need the same secret key for signing
// and validating.
var hmacSecret = []byte(`patricksecretkey`)

func NewTokenHMAC(userId string) (tokenString string, err error) {
	now := time.Now()
	expiredAt := now.Add(time.Hour * 1)

	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": userId,
		"nbf": now.Unix(),
		"exp": expiredAt.Unix(),
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err = token.SignedString(hmacSecret)

	return
}

func VerifyTokenHMAC(tokenString string) (bool) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		
		// hmacSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return hmacSecret, nil
	})

	if err != nil {
		log.Print(err)
		return false
	}	

	return token.Valid
}

type AuthResult struct {
	// TokenString string		`json:"tokenString"`
	Token string		`json:"token"`
}

func Bind(userId string, userPwd string) (string, error){
    l, err := ldap.DialURL("ldap://127.0.0.1:389")
    if err != nil {
        log.Fatal(err)
    }
    defer l.Close()

    userDN := fmt.Sprintf(my_ldap.STAFF_DN, userId)

    err = l.Bind(userDN, userPwd)
    if err != nil {
		// ldapError := err.(*ldap.Error)
		// fmt.Printf("Bind Error Result Code: %d, Bind Error: %s \n", 
		// 		ldapError.ResultCode, ldap.LDAPResultCodeMap[ldapError.ResultCode])
	    return "", err
	}
	
	tokenString, err := NewTokenHMAC(userId)
	if err != nil {
		return "", err
	}
    return tokenString, nil
}

func BindHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "authorization")				

	//TODO: This line is an ad-hoc solution to remove the 
	//      std output triggered by CORS preflighted request via http OPTIONS.
	//      The correct solution is to add a reverse-proxy server to the
	//      main server so that all requests are channeled to this reverse proxy
	//      in order to overcome CORS restriction.
	if (r.Method == "OPTIONS") { return }    

	r.ParseForm()
	fmt.Println("[BindHandler] Request Form Data Received!\n")
	fmt.Println(r.PostForm)
	
	userId := r.PostForm["userId"][0]
	userPwd := r.PostForm["userPwd"][0]		
        
	tokenString, err := Bind(userId, userPwd)
	if err != nil {
		ldapError, ok := err.(*ldap.Error)
		if !ok {
			log.Print(err)
			w.WriteHeader(500)
			fmt.Fprintf(w, "Internal server error!")
			return
		}
		if ldapError.ResultCode == 49 {
			log.Print(err)
			w.WriteHeader(401)
			fmt.Fprintf(w, "Login Unauthorized!")
			return
		}
	}
	
	authResult := AuthResult{
		// TokenString: tokenString,
		Token: tokenString,
	}
	authResultJson, err := json.MarshalIndent(&authResult, "", "\t")
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}

	// w.WriteHeader(200)
	fmt.Fprintf(w, "%s", authResultJson)

	fmt.Printf("Login Successful! \n")
	fmt.Printf("tokenString: %s \n", authResultJson)
}

type User struct {
	Id string			`json:"id"`
	Pwd string			`json:"pwd"`
	Name string 		`json:"name"`
	Address string 		`json:"address"`
	Telephone string 	`json:"telephone"`
	Email string 		`json:"email"`
}

// CreateAccResCode will be 0 if successful in 
// creating new user account in LDAP.
// It will be 1 if the user account entry already
// exist in LDAP.
type CreateAccountResult struct {	
	CreateAccResCode int		`json:"createAccResCode"`
}

func CreateAccount(newUser User) (int, error) {
    l, err := ldap.DialURL("ldap://127.0.0.1:389")
    if err != nil {
		log.Print(err)
		return -1, err
    }
    defer l.Close()
	
	// TODO: Use other less priviledged user
	//       account to bind to LDAP.
	err = l.Bind(my_ldap.DIR_MGR_DN, my_ldap.DIR_MGR_PWD)
    if err != nil {
		log.Print(err)
		return -1, err
	}
		
	newUserDN := fmt.Sprintf(my_ldap.USER_DN, newUser.Id)
    addReq := ldap.NewAddRequest(
		newUserDN, 
		nil,
	)
	addReq.Attribute("objectClass", []string{"top", "person", 
											"organizationalPerson", "inetOrgPerson"})
    addReq.Attribute("cn", []string{newUser.Id})
	addReq.Attribute("sn", []string{newUser.Id})
	addReq.Attribute("userPassword", []string{newUser.Pwd})
	addReq.Attribute("displayName", []string{newUser.Name})
	addReq.Attribute("registeredAddress", []string{newUser.Address})
	addReq.Attribute("telephoneNumber", []string{newUser.Telephone})
	addReq.Attribute("mail", []string{newUser.Email})

    err = l.Add(addReq)
    if err != nil {	
		log.Print(err)

		ldapError := err.(*ldap.Error)
		fmt.Printf("Bind Error Result Code: %d, Bind Error: %s \n", 
			ldapError.ResultCode, ldap.LDAPResultCodeMap[ldapError.ResultCode])		
		// LDAPResultEntryAlreadyExists = 68
		if ldapError.ResultCode == 68 {
			return 1, nil
		}
		return -1, err
	}
	
	return 0, nil
}

func CreateAccountHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "authorization")				

	//TODO: This line is an ad-hoc solution to remove the 
	//      std output triggered by CORS preflighted request via http OPTIONS.
	//      The correct solution is to add a reverse-proxy server to the
	//      main server so that all requests are channeled to this reverse proxy
	//      in order to overcome CORS restriction.
	if (r.Method == "OPTIONS") { return }    

	r.ParseForm()
	fmt.Println("[CreateAccountHandler] Request Form Data Received!\n")
	fmt.Println(r.PostForm)
	
	user := User{}
	user.Id = r.PostForm["userId"][0]
	user.Pwd = r.PostForm["userPwd"][0]		
	user.Name = r.PostForm["userName"][0]		
	user.Address = r.PostForm["userAddress"][0]		
	user.Telephone = r.PostForm["userTelephone"][0]		
	user.Email = r.PostForm["userEmail"][0]		
		
	createAccResCode, err := CreateAccount(user)
	if err != nil {		
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return				
	}	
	createAccountResult := CreateAccountResult{ CreateAccResCode: createAccResCode }

	createAccountResultJson, err := json.MarshalIndent(&createAccountResult, "", "\t")
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}

	fmt.Fprintf(w, "%s", createAccountResultJson)
	fmt.Printf("createAccountResult: %s \n", createAccountResultJson)
}