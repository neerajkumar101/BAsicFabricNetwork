/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

// ============================================================================================================================
// Asset Definitions - The ledger will store marbles and owners
// ============================================================================================================================

const STATE_DOCUMENT_CREATED = 1
const STATE_DOCUMENT_VERIFIED = 2
const STATE_DOCUMENT_REJECTED = 3

type Document struct {
	ObjectType     string `json:"docType"`    //docType is used to distinguish the various types of objects in state database
	DocumentID     string `json:"documentID"` //the fieldtags are needed to keep case from bouncing around
	DocumentString string `json:"documentString"`
	DocumentStatus string `json:"documentStatus"`
	CreatedON      string `json:"createdOn"`
}

// ============================================================================================================================
// Main
// ============================================================================================================================
func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode - %s", err)
	}
}

// ============================================================================================================================
// Init - initialize the chaincode
//
// Marbles does not require initialization, so let's run a simple test instead.
//
// Shows off PutState() and how to pass an input argument to chaincode.
// Shows off GetFunctionAndParameters() and GetStringArgs()
// Shows off GetTxID() to get the transaction ID of the proposal
//
// Inputs - Array of strings
//  ["314"]
//
// Returns - shim.Success or error
// ============================================================================================================================
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	fmt.Println("Marbles Is Starting Up")
	funcName, args := stub.GetFunctionAndParameters()
	var number int
	var err error
	txId := stub.GetTxID()

	fmt.Println("Init() is running")
	fmt.Println("Transaction ID:", txId)
	fmt.Println("  GetFunctionAndParameters() function:", funcName)
	fmt.Println("  GetFunctionAndParameters() args count:", len(args))
	fmt.Println("  GetFunctionAndParameters() args found:", args)

	// expecting 1 arg for instantiate or upgrade
	if len(args) == 1 {
		fmt.Println("  GetFunctionAndParameters() arg[0] length", len(args[0]))

		// expecting arg[0] to be length 0 for upgrade
		if len(args[0]) == 0 {
			fmt.Println("  Uh oh, args[0] is empty...")
		} else {
			fmt.Println("  Great news everyone, args[0] is not empty")

			// convert numeric string to integer
			number, err = strconv.Atoi(args[0])
			if err != nil {
				return shim.Error("Expecting a numeric string argument to Init() for instantiate")
			}

			// this is a very simple test. let's write to the ledger and error out on any errors
			// it's handy to read this right away to verify network is healthy if it wrote the correct value
			err = stub.PutState("selftest", []byte(strconv.Itoa(number)))
			if err != nil {
				return shim.Error(err.Error()) //self-test fail
			}
		}
	}

	// showing the alternative argument shim function
	alt := stub.GetStringArgs()
	fmt.Println("  GetStringArgs() args count:", len(alt))
	fmt.Println("  GetStringArgs() args found:", alt)

	// store compatible marbles application version
	err = stub.PutState("marbles_ui", []byte("4.0.1"))
	if err != nil {
		return shim.Error(err.Error())
	}

	fmt.Println("Ready for action") //self-test pass
	return shim.Success(nil)
}

// ============================================================================================================================
// Invoke - Our entry point for Invocations
// ============================================================================================================================
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println(" ")
	fmt.Println("starting invoke, for - " + function)

	// Handle different functions
	if function == "init" { //initialize the chaincode state, used as reset
		return t.Init(stub)
	} else if function == "read" { //generic read ledger
		return read(stub, args)
	} else if function == "write" { //generic writes to ledger
		return write(stub, args)
	} else if function == "init_document" { //create a new marble
		return init_document(stub, args)
	} else if function == "read_everything" { //read everything, (owners + marbles + companies)
		return read_everything(stub)
	} else if function == "getHistory" { //read history of a marble (audit)
		return getHistory(stub, args)
	} else if function == "update_document" { //create a new marble
		return update_document(stub, args)
	}

	// error out
	fmt.Println("Received unknown invoke function name - " + function)
	return shim.Error("Received unknown invoke function name - '" + function + "'")
}

// ============================================================================================================================
// Query - legacy function
// ============================================================================================================================
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Error("Unknown supported call - Query()")
}

// ============================================================================================================================
// Read - read a generic variable from ledger
//
// Shows Off GetState() - reading a key/value from the ledger
//
// Inputs - Array of strings
//  0
//  key
//  "abc"
//
// Returns - string
// ============================================================================================================================
func read(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var key, jsonResp string
	var err error
	fmt.Println("starting read")

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting key of the var to query")
	}

	// input sanitation
	err = sanitize_arguments(args)
	if err != nil {
		return shim.Error(err.Error())
	}

	key = args[0]
	valAsbytes, err := stub.GetState(key) //get the var from ledger
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + key + "\"}"
		return shim.Error(jsonResp)
	}

	fmt.Println("- end read")
	return shim.Success(valAsbytes) //send it onward
}

// ============================================================================================================================
// Get everything we need (owners + marbles + companies)
//
// Inputs - none
//
// Returns:
// {
//	"owners": [{
//			"id": "o99999999",
//			"company": "United Marbles"
//			"username": "alice"
//	}],
//	"marbles": [{
//		"id": "m1490898165086",
//		"color": "white",
//		"docType" :"marble",
//		"owner": {
//			"company": "United Marbles"
//			"username": "alice"
//		},
//		"size" : 35
//	}]
// }
// ============================================================================================================================
func read_everything(stub shim.ChaincodeStubInterface) pb.Response {
	type Everything struct {
		Documents []Document `json:"documents"`
	}
	var everything Everything

	// ---- Get All Marbles ---- //
	resultsIterator, err := stub.GetStateByRange("m0", "m9999999999999999999")
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	for resultsIterator.HasNext() {
		aKeyValue, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		queryKeyAsStr := aKeyValue.Key
		queryValAsBytes := aKeyValue.Value
		fmt.Println("on document id - ", queryKeyAsStr)
		var document Document
		json.Unmarshal(queryValAsBytes, &document)                    //un stringify it aka JSON.parse()
		everything.Documents = append(everything.Documents, document) //add this marble to the list
	}
	fmt.Println("documents array - ", everything.Documents)

	//change to array of bytes
	everythingAsBytes, _ := json.Marshal(everything) //convert to array of bytes
	return shim.Success(everythingAsBytes)
}

// ============================================================================================================================
// Get history of asset
//
// Shows Off GetHistoryForKey() - reading complete history of a key/value
//
// Inputs - Array of strings
//  0
//  id
//  "m01490985296352SjAyM"
// ============================================================================================================================
func getHistory(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	type AuditHistory struct {
		TxId  string   `json:"txId"`
		Value Document `json:"value"`
	}
	var history []AuditHistory
	var document Document

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	documentId := args[0]
	fmt.Printf("- start getHistoryForMarble: %s\n", documentId)

	// Get History
	resultsIterator, err := stub.GetHistoryForKey(documentId)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	for resultsIterator.HasNext() {
		historyData, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}

		var tx AuditHistory
		tx.TxId = historyData.TxId                   //copy transaction id over
		json.Unmarshal(historyData.Value, &document) //un stringify it aka JSON.parse()
		if historyData.Value == nil {                //marble has been deleted
			var emptyDocument Document
			tx.Value = emptyDocument //copy nil marble
		} else {
			json.Unmarshal(historyData.Value, &document) //un stringify it aka JSON.parse()
			tx.Value = document                          //copy marble over
		}
		history = append(history, tx) //add this tx to the list
	}
	fmt.Printf("- getHistoryForDocument returning:\n%s", history)

	//change to array of bytes
	historyAsBytes, _ := json.Marshal(history) //convert to array of bytes
	return shim.Success(historyAsBytes)
}

// ============================================================================================================================
// Get Marble - get a marble asset from ledger
// ============================================================================================================================
func get_document(stub shim.ChaincodeStubInterface, id string) (Document, error) {
	doc := Document{}
	documentAsBytes, err := stub.GetState(id) //getState retreives a key/value from the ledger
	if err == nil {                           //this seems to always succeed, even if key didn't exist
		return doc, errors.New("Failed to find marble - " + id)
	}

	/*fmt.Println("document id from document is " + document.DocumentID)
	fmt.Println("document id of requested document is " + id)*/

	if documentAsBytes == nil { //test if marble is actually here or just nil
		return doc, errors.New("Document does not exist - " + id)
	}

	err = json.Unmarshal([]byte(documentAsBytes), &doc)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
		return doc, errors.New("unable to unmarshall")
	}

	fmt.Println(doc)
	return doc, nil
}

// ========================================================
// Input Sanitation - dumb input checking, look for empty strings
// ========================================================
func sanitize_arguments(strs []string) error {
	for i, val := range strs {
		if len(val) <= 0 {
			return errors.New("Argument " + strconv.Itoa(i) + " must be a non-empty string")
		}
		// if len(val) > 32 {
		// 	return errors.New("Argument " + strconv.Itoa(i) + " must be <= 32 characters")
		// }
	}
	return nil
}

// ============================================================================================================================
// write() - genric write variable into ledger
//
// Shows Off PutState() - writting a key/value into the ledger
//
// Inputs - Array of strings
//    0   ,    1
//   key  ,  value
//  "abc" , "test"
// ============================================================================================================================
func write(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var key, value string
	var err error
	fmt.Println("starting write")

	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting 2. key of the variable and value to set")
	}

	// input sanitation
	err = sanitize_arguments(args)
	if err != nil {
		return shim.Error(err.Error())
	}

	key = args[0] //rename for funsies
	value = args[1]
	err = stub.PutState(key, []byte(value)) //write the variable into the ledger
	if err != nil {
		return shim.Error(err.Error())
	}

	fmt.Println("- end write")
	return shim.Success(nil)
}

// ============================================================================================================================
// Init Marble - create a new marble, store into chaincode state
//
// Shows off building a key's JSON value manually
//
// Inputs - Array of strings
//      0      ,    1  ,  2  ,      3          ,       4
//     id      ,  color, size,     owner id    ,  authing company
// "m999999999", "blue", "35", "o9999999999999", "united marbles"
// ============================================================================================================================
func init_document(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error
	fmt.Println("starting init_document")

	//input sanitation
	err1 := sanitize_arguments(args)
	if err1 != nil {
		return shim.Error("Cannot sanitize arguments")
	}

	document_ID := args[1]
	document_Status := args[3]
	//check if marble id already exists
	documentAsBytes, err := stub.GetState(document_ID)
	if err != nil { //this seems to always succeed, even if key didn't exist
		return shim.Error("error in finding document for - " + document_ID)
	}
	if documentAsBytes != nil {
		fmt.Println("This document already exists - " + document_ID)
		return shim.Error("This document already exists - " + document_ID) //all stop a marble by this id exists
	}

	if document_Status != strconv.Itoa(STATE_DOCUMENT_CREATED) {
		return shim.Error(" Invalid document status - " + document_Status + "Expecting document status unverified - 1") //all stop a marble by this id exists
	}

	documentObject, err := CreateDocumentObject(args[0:])
	if err != nil {
		errorStr := "initDocument() : Failed Cannot create object buffer for write : " + args[0]
		fmt.Println(errorStr)
		return shim.Error(errorStr)
	}

	fmt.Println(documentObject)
	buff, err := DOCtoJSON(documentObject)
	if err != nil {
		return shim.Error("unable to convert document to json")
	}

	err = stub.PutState(document_ID, buff) //store marble with id as key
	if err != nil {
		return shim.Error(err.Error())
	}

	fmt.Println("- end init_document")
	return shim.Success(nil)
}

// CreateAssetObject creates an asset
func CreateDocumentObject(args []string) (Document, error) {
	var myDocument Document

	// Check there are 3 Arguments provided as per the the struct
	if len(args) != 4 {
		fmt.Println("CreateDocumentObject(): Incorrect number of arguments. Expecting 4 ")
		return myDocument, errors.New("CreateDocumentObject(): Incorrect number of arguments. Expecting 4 ")
	}

	myDocument = Document{args[0], args[1], args[2], args[3], time.Now().Format("20060102150405")}
	return myDocument, nil
}

func DOCtoJSON(doc Document) ([]byte, error) {

	djson, err := json.Marshal(doc)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return djson, nil
}

func JSONtoDoc(data []byte) (Document, error) {

	doc := Document{}
	err := json.Unmarshal([]byte(data), &doc)
	if err != nil {
		fmt.Println("Unmarshal failed : ", err)
		return doc, err
	}

	return doc, nil
}

func getQueryResultForQueryString(stub shim.ChaincodeStubInterface, queryString string) ([]byte, error) {

	fmt.Printf("- getQueryResultForQueryString queryString:\n%s\n", queryString)

	resultsIterator, err := stub.GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing QueryRecords
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"Key\":")
		buffer.WriteString("\"")
		buffer.WriteString(queryResponse.Key)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Record\":")
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- getQueryResultForQueryString queryResult:\n%s\n", buffer.String())

	return buffer.Bytes(), nil
}

// ============================================================================================================================
// Init Marble - create a new marble, store into chaincode state
//
// Shows off building a key's JSON value manually
//
// Inputs - Array of strings
//      0      ,    1  ,  2  ,      3          ,       4
//     id      ,  color, size,     owner id    ,  authing company
// "m999999999", "blue", "35", "o9999999999999", "united marbles"
// ============================================================================================================================
func update_document(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error
	fmt.Println("starting init_document")

	if len(args) != 3 {
		return shim.Error("Incorrect number of arguments. Expecting 3")
	}

	//input sanitation
	err = sanitize_arguments(args)
	if err != nil {
		return shim.Error(err.Error())
	}

	document_ID := args[0]
	document_Status := args[2]

	documentAsBytes, err := stub.GetState(document_ID)
	str := fmt.Sprintf("%s", documentAsBytes)
	fmt.Println("string is " + str)

	if err != nil { //this seems to always succeed, even if key didn't exist
		fmt.Println("Error in finding Document - " + document_ID)
		return shim.Error("error in finding document for - " + document_ID)
	}
	/*if len(documentAsBytes) == 0 {
		fmt.Println("This document already exists and is null- " + document_ID)
		return shim.Error("This document already exists but is null - " + document_ID) //all stop a marble by this id exists
	}*/

	list := []string{strconv.Itoa(STATE_DOCUMENT_REJECTED), strconv.Itoa(STATE_DOCUMENT_VERIFIED)}

	if !(stringInSlice(document_Status, list)) {
		return shim.Error(" Invalid document update status - " + document_Status + "Expecting document status verified/Rejected - 2 || 3") //all stop a marble by this id exists
	}

	dat, err := JSONtoDoc(documentAsBytes)
	if err != nil {
		return shim.Error("unable to convert jsonToDoc for" + document_ID)
	}

	fmt.Println(dat)
	fmt.Println(dat.DocumentID)

	updatedDocument := Document{dat.ObjectType, dat.DocumentID, dat.DocumentString, document_Status, time.Now().Format("20060102150405")}

	buff, err := DOCtoJSON(updatedDocument)
	if err != nil {
		errorStr := "updateDispatchOrder() : Failed Cannot create object buffer for write : " + args[1]
		fmt.Println(errorStr)
		return shim.Error(errorStr)
	}

	err = stub.PutState(dat.DocumentID, buff) //store marble with id as key
	if err != nil {
		return shim.Error(err.Error())
	}

	fmt.Println("- end init_document")
	return shim.Success(nil)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
