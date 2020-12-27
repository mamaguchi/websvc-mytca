package my_mongodb

import (
	"go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
	// "go.mongodb.org/mongo-driver/mongo/readpref"
	// "go.mongodb.org/mongo-driver/mongo/writeconcern"
	"fmt"
	"net/http"
	"time"
	"context"
	"log"
	"encoding/json"
	"strconv"
	"mytca/booking/db/my_ldap"
)

type ClinicSvcBookings struct {
	MonthlyOpSchedules []DailyOpSchedule `bson:"monthlyOpSchedule"`
}

type DailyOpSchedule struct {
	DayOfWeek int				`bson:"dayOfWeek" json:"dayOfWeek"`
	QueuesCapPerDay []int 		`bson:"queuesCapPerDay" json:"queuesCapPerDay"` 
	QueuesUsgPerDay []int 		`bson:"queuesUsgPerDay" json:"queuesUsgPerDay"` 
}

func GetSvcBookingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "authorization")	
	r.ParseForm()

	//TODO: This line is an ad-hoc solution to remove the 
	//      std output triggered by CORS preflighted request via http OPTIONS.
	//      The correct solution is to add a reverse-proxy server to the
	//      main server so that all requests are channeled to this reverse proxy
	//      in order to overcome CORS restriction.
	if (r.Method == "OPTIONS") { return }

	fmt.Println("[GetSvcBookingsHandler] Request Form Data Received!\n")
	fmt.Println(r.PostForm)

	clinicId := r.PostForm["clinicId"][0]
	svc := r.PostForm["service"][0]
	date := r.PostForm["date"][0]
	dept := my_ldap.SvcToDeptMap[svc]

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

	dailyOpScheduleProjPath := "monthlyOpSchedule"
	projection := bson.D{
		{dailyOpScheduleProjPath, 
			bson.D{
				{"$elemMatch", bson.D{{"date", date}}},
			},
		},		
	}

	opts := options.FindOne()
	opts.SetProjection(projection)
	var clinicSvcBookings ClinicSvcBookings
	var dailyOpSchedule DailyOpSchedule
	err = bookingColl.FindOne(
		ctx,
		bson.D{
			{"clinicId" , clinicId},
			{"dept", dept},
			{"date", date[:7]},
			{"monthlyOpSchedule.date", date},
		},
		opts,
	).Decode(&clinicSvcBookings)
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	dailyOpSchedule = clinicSvcBookings.MonthlyOpSchedules[0]
	fmt.Printf("DailyOpSchedule: %+v \n", dailyOpSchedule)

	dailyOpScheduleJson, err := json.MarshalIndent(&dailyOpSchedule, "", "\t")
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Fprintf(w, "%s", dailyOpScheduleJson)
}

type PatientBooking struct {
	Date string					`bson:"date" json:"date"`
	ClinicId string 			`bson:"clinicId" json:"clinicId"` 
	ClinicName string 			`bson:"clinicName" json:"clinicName"`
	BookedService string		`bson:"bookedService" json:"bookedService"` 
	BookedTime string			`bson:"bookedTime" json:"bookedTime"`
}

// This func handles the booking transaction made by patients.
func MakeBookingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "authorization")	
	r.ParseForm()

	//TODO: This line is an ad-hoc solution to remove the 
	//      std output triggered by CORS preflighted request via http OPTIONS.
	//      The correct solution is to add a reverse-proxy server to the
	//      main server so that all requests are channeled to this reverse proxy
	//      in order to overcome CORS restriction.
	if (r.Method == "OPTIONS") { return }

	fmt.Println("[MakeBookingHandler] Request Form Data Received!\n")
	fmt.Println(r.PostForm)

	userId := r.PostForm["userId"][0]
	clinicId := r.PostForm["clinicId"][0]
	clinicName := r.PostForm["clinicName"][0]
	svc := r.PostForm["service"][0]
	date := r.PostForm["date"][0]
	opHrIdx := r.PostForm["opHrIdx"][0]
	dept := my_ldap.SvcToDeptMap[svc]

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

	dailyOpScheduleProjPath := "monthlyOpSchedule"
	queuesUsgPerHrPath := fmt.Sprintf("monthlyOpSchedule.$.queuesUsgPerDay.%s", opHrIdx)
	bookingsPath := fmt.Sprintf("monthlyOpSchedule.$.queuesPerDay.%s.bookings", opHrIdx)
	// patientIdsPath := fmt.Sprintf("monthlyOpSchedule.$.queuesPerDay.%s.patientIds", opHrIdx)
	// bookingReasonsPath := fmt.Sprintf("monthlyOpSchedule.$.queuesPerDay.%s.bookingReasons", opHrIdx)

	// Step 1: Define the callback that specifies the sequence of operations to perform inside the transaction.
	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Important: You must pass sessCtx as the Context parameter to the operations for them to be executed in the
		// transaction.
		queueUsgIncrement := 100		

		projection := bson.D{
			{dailyOpScheduleProjPath, 
				bson.D{
					{"$elemMatch", bson.D{{"date", date}}},
				},
			},		
		}

		opts := options.FindOneAndUpdate()
		opts.SetProjection(projection)
		opts.SetReturnDocument(options.After)
		var updatedDocument ClinicSvcBookings
		booking := Booking{
			PatientId: userId,
			BookedService: svc,
		}
		err = bookingColl.FindOneAndUpdate(
			sessCtx,
			bson.D{
				{"clinicId" , clinicId},
				{"dept", dept},
				{"date", date[:7]},
				{"monthlyOpSchedule.date", date},
			},
			bson.D{
				{"$inc", bson.D{{queuesUsgPerHrPath, queueUsgIncrement}}},
				{"$push", bson.D{{bookingsPath, booking}}},
				// {"$push", bson.D{{patientIdsPath, userId}}},
				// {"$push", bson.D{{bookingReasonsPath, svc}}},
			},
			opts,
		).Decode(&updatedDocument)
		if err != nil {
			// ErrNoDocuments means that the filter did not match any documents in the collection
			if err == mongo.ErrNoDocuments {
				sessCtx.AbortTransaction(sessCtx)
				return "ErrNoDocuments, Transaction Rollbacked!", err
			}
			sessCtx.AbortTransaction(sessCtx)
			return "Transaction Rollbacked!", err
		}

		queueCapIdx, err := strconv.Atoi(opHrIdx)
		if err != nil {
			sessCtx.AbortTransaction(sessCtx)
			return "Transaction Rollbacked!", err
		}
		updatedQueueCap := updatedDocument.MonthlyOpSchedules[0].QueuesCapPerDay[queueCapIdx]
		updatedQueueUsg := updatedDocument.MonthlyOpSchedules[0].QueuesUsgPerDay[queueCapIdx]
		fmt.Printf("Updated QueuesUsgPerDay to: %v\n", updatedQueueUsg)

		if updatedQueueUsg > updatedQueueCap {
			fmt.Println("Insufficient Queue Capacity, rolling back transaction...")
			sessCtx.AbortTransaction(sessCtx)
			return "Transaction Rollbacked!", err
		}

		// Now insert a booking record into Mongodb for patient.
		patientBookingsColl := mytcaDB.Collection("patientBookings")
		patientBooking := PatientBooking{
			Date: date,
			ClinicId: clinicId,
			ClinicName: clinicName,
			BookedService: svc,
			BookedTime: opHrIdx,
		}
		optsUpdate := options.Update()
		optsUpdate.SetUpsert(true)
		res, err := patientBookingsColl.UpdateOne(
			sessCtx, 
			bson.D{
				{"patientId" , userId},
			},			
			bson.D{
				{"$push", bson.D{{"bookings", patientBooking}}},
			},
			optsUpdate,
		)
		if err != nil || (res.UpsertedCount == 0 && res.ModifiedCount == 0){
			log.Printf("Error: %v \n", err)
			log.Printf("UpsertedCount: %v \nModifiedCount: %v \n", res.UpsertedCount, res.ModifiedCount)
			sessCtx.AbortTransaction(sessCtx)
			return "Patient booking record upsert failed. Transaction Rollbacked!", err
		}

		return "Transaction Successful!", nil
	}

	// Step 2: Start a session and run the callback using WithTransaction.
	session, err := client.StartSession()
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	defer session.EndSession(ctx)
	result, err := session.WithTransaction(ctx, callback)
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Printf("Transaction Returned Result: %v\n", result)

	// Write result back to web service client
	resultJson := struct{BookingRes string `json:"bookingRes"`}{
		BookingRes: result.(string),
	}
	outputJson, err := json.MarshalIndent(&resultJson, "", "\t")
	if err != nil {
		fmt.Printf("Json Encoding Error: %v", err)
		return
	}
	fmt.Fprintf(w, "%s", outputJson)
}

type PatientBookings struct {
	Bookings []PatientBooking	`bson:"bookings" json:"bookings"`
}

func GetPatientBookingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "authorization")	

	//TODO: This line is an ad-hoc solution to remove the 
	//      std output triggered by CORS preflighted request via http OPTIONS.
	//      The correct solution is to add a reverse-proxy server to the
	//      main server so that all requests are channeled to this reverse proxy
	//      in order to overcome CORS restriction.
	if (r.Method == "OPTIONS") { return }

	r.ParseForm()
	fmt.Println("[GetPatientBookingsHandler] Request Form Data Received!\n")
	fmt.Println(r.Form)

	patientId := r.Form["patientId"][0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()	
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	defer func() {
		if err=client.Disconnect(ctx); err!=nil {
			panic(err)
		}
	}()
	mytcaDB := client.Database("test")
	ptBookingColl := mytcaDB.Collection("patientBookings")

	projection := bson.D{
		{"bookings", 1},	
	}
	opts := options.Find()
	opts.SetProjection(projection)
	
	cursor, err := ptBookingColl.Find(
		ctx,
		bson.D{
			{"patientId" , patientId},			
		},
		opts,
	)
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}

	var patientBookings []PatientBookings
	if err = cursor.All(ctx, &patientBookings); err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Printf("PatientBookings: %+v \n", patientBookings[0])

	patientBookingsJson, err := json.MarshalIndent(&patientBookings[0], "", "\t")
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Fprintf(w, "%s", patientBookingsJson)
}

// Remove a booking from both clinic and patient records
func DelBookingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "authorization")	
	r.ParseForm()

	//TODO: This line is an ad-hoc solution to remove the 
	//      std output triggered by CORS preflighted request via http OPTIONS.
	//      The correct solution is to add a reverse-proxy server to the
	//      main server so that all requests are channeled to this reverse proxy
	//      in order to overcome CORS restriction.
	if (r.Method == "OPTIONS") { return }

	fmt.Println("[DelBookingHandler] Request Form Data Received!\n")
	fmt.Println(r.PostForm)

	patientId := r.PostForm["patientId"][0]
	clinicId := r.PostForm["clinicId"][0]
	svc := r.PostForm["service"][0]
	date := r.PostForm["date"][0]
	opHrIdx := r.PostForm["opHrIdx"][0]
	dept := my_ldap.SvcToDeptMap[svc]

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

	queuesUsgPerHrPath := fmt.Sprintf("monthlyOpSchedule.$.queuesUsgPerDay.%s", opHrIdx)
	bookingsPath := fmt.Sprintf("monthlyOpSchedule.$.queuesPerDay.%s.bookings", opHrIdx)

	// Step 1: Define the callback that specifies the sequence of operations to perform inside the transaction.
	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Important: You must pass sessCtx as the Context parameter to the operations for them to be executed in the
		// transaction.
		queueUsgDecrement := -100

		booking := bson.D{
			{"patientId", patientId},
			{"bookedService", svc},
		}
		res, err := bookingColl.UpdateOne(
			sessCtx,
			bson.D{
				{"clinicId" , clinicId},
				{"dept", dept},
				{"date", date[:7]},
				{"monthlyOpSchedule.date", date},
			},
			bson.D{
				{"$inc", bson.D{{queuesUsgPerHrPath, queueUsgDecrement}}},
				{"$pull", bson.D{{bookingsPath, booking}}},
				
			},			
		)
		if err != nil {
			log.Print(err)
			sessCtx.AbortTransaction(sessCtx)
			return "Error during deleting a clinic booking record. Transaction Rollbacked!", err
		}
		if res.ModifiedCount == 0 {
			log.Printf("ModifiedCount: %v \n", res.ModifiedCount)
			sessCtx.AbortTransaction(sessCtx)
			return "Error during deleting a clinic booking record. Transaction Rollbacked!", err
		}

		// Now delete a booking record from patient records in MongoDB.
		patientBookingsColl := mytcaDB.Collection("patientBookings")
		patientBooking := bson.D{
			{"date", date},
			{"clinicId", clinicId},
			{"bookedService", svc},
			{"bookedTime", opHrIdx},
		}
		res, err = patientBookingsColl.UpdateOne(
			sessCtx, 
			bson.D{
				{"patientId" , patientId},
			},			
			bson.D{
				{"$pull", bson.D{{"bookings", patientBooking}}},
			},
		)
		if err != nil {
			log.Print(err)
			sessCtx.AbortTransaction(sessCtx)
			return "Error during deleting a patient booking record. Transaction Rollbacked!", err
		}
		if res.ModifiedCount == 0 {
			log.Printf("ModifiedCount: %v \n", res.ModifiedCount)
			sessCtx.AbortTransaction(sessCtx)
			return "Error during deleting a patient booking record. Transaction Rollbacked!", err
		}

		return "Transaction Successful!", nil
	}

	// Step 2: Start a session and run the callback using WithTransaction.
	session, err := client.StartSession()
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	defer session.EndSession(ctx)
	result, err := session.WithTransaction(ctx, callback)
	if err != nil {
		log.Print(err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal server error!")
		return
	}
	fmt.Printf("Transaction Returned Result: %v\n", result)

	// Write result back to web service client
	resultJson := struct{DelBookingRes string `json:"delBookingRes"`}{
		DelBookingRes: result.(string),
	}
	outputJson, err := json.MarshalIndent(&resultJson, "", "\t")
	if err != nil {
		fmt.Printf("Json Encoding Error: %v", err)
		return
	}
	fmt.Fprintf(w, "%s", outputJson)
}