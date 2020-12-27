package my_ldap

const (
	DIR_MGR_DN       	=	"cn=Directory Manager"
	DIR_MGR_PWD    	 	=	"88motherfaker88"
	USER_BASE_DN     	=	"ou=people,dc=example,dc=com"
	USER_DN     		=	"cn=%s,ou=people,dc=example,dc=com"
	STAFF_BASE_DN     	=	"ou=people,dc=example,dc=com"
	STAFF_DN 			=	"staffId=%s," + STAFF_BASE_DN
	CLINIC_BASE_DN    	=   "ou=pkd_%s,ou=jkn_%s,ou=kkm-clinic,ou=groups,dc=example,dc=com"
	CLINIC_DN   		=   "cn=%s," + CLINIC_BASE_DN
	DEPT_BASE_DN   		=   "ou=dept," + CLINIC_DN
	DEPT_DN   			=   "clinicDeptName=%s," + DEPT_BASE_DN
	SERVICE_BASE_DN	 	=   "ou=service," + DEPT_DN
	SERVICE_DN	 		=   "clinicServiceName=%s," + SERVICE_BASE_DN
)