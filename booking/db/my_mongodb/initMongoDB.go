package my_mongodb

import (
	"go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
	"fmt"
	"time"
	"context"
	"log"
	"strconv"
	"strings"
	"mytca/booking/db/my_ldap"
	// "github.com/go-ldap/ldap/v3"
    // "go.mongodb.org/mongo-driver/mongo/readpref"
	//"net/http"
	//"encoding/json"
)



// MISC FUNC
// =========
func aggregateClinicSvcAvaiDays(clinicSvcAvaiDays []string) ([7]int) {
	type bits uint8
	var flag bits
	for _, str := range clinicSvcAvaiDays {
	    strSplit := strings.Split(str, ",")
	    for i:=0; i<len(strSplit); i++ {
            var val bits
            tmp, _ := strconv.Atoi(strSplit[i])
            val = bits(tmp)
            flag = flag | val << i
	    }	
	}
	// fmt.Printf("Avai Days Flag(reversed): %#b \n", flag)
	
	var aggrAvaiDays [7]int
	for k:=0; k<7; k++ {
	    aggrAvaiDays[k] = int(1 & (flag >> k))
	}
	return aggrAvaiDays
}


// MAIN
// ====
type InitDailyOpSchedule struct {
	Date string 				`bson:"date" json:"date"`
	DayOfWeek int				`bson:"dayOfWeek" json:"dayOfWeek"`
	QueuesCapPerDay []int 		`bson:"queuesCapPerDay" json:"queuesCapPerDay"` 
	QueuesUsgPerDay []int 		`bson:"queuesUsgPerDay" json:"queuesUsgPerDay"` 
	QueuesPerDay []QueuePerHr   `bson:"queuesPerDay" json:"queuesPerDay"` 
}

type QueuePerHr struct {
	Bookings []Booking 			`bson:"bookings" json:"bookings"`	
}

type Booking struct {
	PatientId string 		   		`bson:"patientId" json:"patientId"`
	BookedService string 			`bson:"bookedService" json:"bookedService"`
	// BookingReason string			`bson:"bookingReason" json:"bookingReason"`
}

func InitOpSchedule(userId string, userPwd string, year int, month int, 
					clinicId string, district string, state string) (error){

	deptDataList, err := my_ldap.GetDeptNameAndStaffNum(userId, userPwd, clinicId, district, state)
	if err != nil {
		log.Print(err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()	
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	defer func() {
		if err=client.Disconnect(ctx); err!=nil {
			panic(err)			
		}
	}()
	mytcaDB := client.Database("test")
	bookingColl := mytcaDB.Collection("booking4")

	for _, deptData := range deptDataList {
		var deptName = deptData.Name 
		var deptNumOfStaff = deptData.NumOfStaff 		

		var mth time.Month = time.Month(month)
		t := time.Date(year, mth, 1, 0,0,0,0,time.UTC)
		lastDayOfMonth := time.Date(year, time.Month(month+1), 0, 0,0,0,0,time.UTC).Day()
		monthlyOpSchedule := []InitDailyOpSchedule{}
		
		for i:=1; i<=lastDayOfMonth; i++ {			
			queuesCapPerDay := []int{} 
			queuesUsgPerDay := []int{} 
			queuesPerDay := []QueuePerHr{}
			queueCapPerHr := deptNumOfStaff * 60
			queueUsgPerHr := 0
			queuePerHr := QueuePerHr{
				Bookings: []Booking{},
			}

			for j:=0; j < 24; j++ {
				queuesCapPerDay = append(queuesCapPerDay, queueCapPerHr)
				queuesUsgPerDay = append(queuesUsgPerDay, queueUsgPerHr)
				queuesPerDay = append(queuesPerDay, queuePerHr)
			}

			initDailyOpSchedule := InitDailyOpSchedule{
				Date: t.String()[:10],
				DayOfWeek: int(t.Weekday()),
				QueuesCapPerDay: queuesCapPerDay,
				QueuesUsgPerDay: queuesUsgPerDay,
				QueuesPerDay: queuesPerDay,
			}
			monthlyOpSchedule = append(monthlyOpSchedule, initDailyOpSchedule)				
			t = t.AddDate(0, 0, 1)
		}

		entryDate := strconv.Itoa(year) + "-" + strconv.Itoa(month)

		res, err := bookingColl.InsertOne(
			ctx, 
			bson.D{
				{"clinicId", clinicId},
				{"dept", deptName},
				{"date", entryDate},
				{"monthlyOpSchedule", monthlyOpSchedule},
			},
		)
		if err != nil {
			log.Print(err)
			return err
		}
		fmt.Printf("The _id field of the inserted document (nil if no insert was done):\t %v\n", res.InsertedID)
	}

	return nil
}