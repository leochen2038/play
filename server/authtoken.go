package server

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func padding(plainText []byte, blockSize int) []byte {
	n := blockSize - len(plainText)%blockSize
	temp := bytes.Repeat([]byte{byte(n)}, n)
	plainText = append(plainText, temp...)
	return plainText
}

func unPadding(cipherText []byte) []byte {
	end := cipherText[len(cipherText)-1]
	cipherText = cipherText[:len(cipherText)-int(end)]
	return cipherText
}

func Encrypt(plainText, key, iv []byte) ([]byte, error) {
	var err error
	var block cipher.Block

	if block, err = aes.NewCipher(key); err != nil {
		return nil, err
	}

	plainText = padding(plainText, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, iv)
	cipherText := make([]byte, len(plainText))
	blockMode.CryptBlocks(cipherText, plainText)

	return cipherText, nil
}

func Decrypt(cipherText, key, iv []byte) ([]byte, error) {
	var err error
	var block cipher.Block

	if block, err = aes.NewCipher(key); err != nil {
		return nil, err
	}

	blockMode := cipher.NewCBCDecrypter(block, iv)
	plainText := make([]byte, len(cipherText))
	blockMode.CryptBlocks(plainText, cipherText)
	plainText = unPadding(plainText)

	return plainText, nil
}

//get 参数处理
func DealGetParam(Param map[string][]string) map[string][]string {
	var token string
	var uid int
	var udid string
	var device int = -1
	var deviceid string
	if v, ok := Param["uid"]; ok {
		uid, _ = strconv.Atoi(v[0])
	}
	if v, ok := Param["udid"]; ok {
		udid = v[0]
	}
	if v, ok := Param["device"]; ok {
		device, _ = strconv.Atoi(v[0])
	}
	if v, ok := Param["deviceid"]; ok {
		deviceid = v[0]
	}
	if v, ok := Param["authToken"]; ok {
		token = v[0]
		uid, udid, device, deviceid = DecodeNew(token)
		if uid == 0 {
			Param["uid"] = []string{"0"}
		} else {
			Param["uid"] = []string{strconv.Itoa(uid)}
		}
		Param["udid"] = []string{udid}
		if device == 0 {
			Param["device"] = []string{"0"}
		} else {
			Param["device"] = []string{strconv.Itoa(device)}
		}
		Param["deviceid"] = []string{deviceid}
	} else {
		token, _ = EncodeNew(uid, udid, device, deviceid)
		Param["pre_token"] = []string{token}
	}
	return Param
}

//post 参数处理
func DealPostParam(request *http.Request) bool {
	var token string
	var uid int
	var udid string
	var device int = -1
	var deviceid string
	uid, _ = strconv.Atoi(request.Form.Get("uid"))
	udid = request.Form.Get("udid")
	if request.Form.Get("device") != "" {
		device, _ = strconv.Atoi(request.Form.Get("device"))
	}
	deviceid = request.Form.Get("deviceid")
	token = request.Form.Get("authToken")
	if token != "" {
		uid, udid, device, deviceid = DecodeNew(token)

		if uid == 0 {
			request.Form.Add("uid", "0")
		} else {
			request.Form.Add("uid", strconv.Itoa(uid))
		}
		request.Form.Add("udid", udid)
		if device == 0 {
			request.Form.Add("device", "0")
		} else {
			request.Form.Add("device", strconv.Itoa(device))
		}
		request.Form.Add("deviceid", deviceid)
	} else if CheckParam(uid, udid, device, deviceid) {
		token, _ = EncodeNew(uid, udid, device, deviceid)
		request.Form.Add("pre_token", token)
	}
	return true
}

//token json参数回填
func ReturnJsonParam(raw []byte) []byte {
	var token string
	var uid int
	var udid string
	var device int = -1
	var deviceid string
	var AllData map[string]interface{}
	if err := json.Unmarshal(raw, &AllData); err != nil {
		return raw
	}
	if v, ok := AllData["h"]; ok {
		ws := v.(map[string]interface{})
		if tokenAuth, ok := ws["authToken"]; ok {
			token = tokenAuth.(string)
			uid, udid, device, deviceid = DecodeNew(token)
			ws["uid"] = uid
			ws["udid"] = udid
			ws["device"] = device
			ws["deviceid"] = deviceid
			raw, _ = json.Marshal(AllData)
			if b, ok := AllData["b"]; ok {
				bType := reflect.TypeOf(b).String()
				if bType == "map[string]interface {}" {
					bdata := b.(map[string]interface{})
					bdata["uid"] = strconv.Itoa(uid)
					bdata["udid"] = udid
					bdata["device"] = strconv.Itoa(device)
					bdata["deviceid"] = deviceid
				}
			}
		} else {
			if ouid, ok := ws["uid"]; ok {
				//fmt.Printf(`%T`, ouid)
				ouidType := reflect.TypeOf(ouid).String()
				if ouidType == "float64" {
					uid = int(ouid.(float64))
				} else if ouidType == "string" {
					uid, _ = strconv.Atoi(ouid.(string))
				}
			}
			if oudid, ok := ws["udid"]; ok {
				oudidType := reflect.TypeOf(oudid).String()
				if oudidType == "float64" {
					udid = strconv.Itoa(int(oudid.(float64)))
				} else if oudidType == "string" {
					udid = oudid.(string)
				}
			}
			if odevice, ok := ws["device"]; ok {
				odeviceType := reflect.TypeOf(odevice).String()
				if odeviceType == "float64" {
					device = int(odevice.(float64))
				} else if odeviceType == "string" {
					device, _ = strconv.Atoi(odevice.(string))
				}
			}
			if odeviceid, ok := ws["deviceid"]; ok {
				odeviceidType := reflect.TypeOf(odeviceid).String()
				if odeviceidType == "float64" {
					deviceid = strconv.Itoa(int(odeviceid.(float64)))
				} else if odeviceidType == "string" {
					deviceid = odeviceid.(string)
				}
			}
		}
		AllData["pre_token"] = ""
		if token == "" && CheckParam(uid, udid, device, deviceid) {
			token, _ = EncodeNew(uid, udid, device, deviceid)
			AllData["pre_token"] = token
		}
	} else if v, ok := AllData["authToken"]; ok {
		token = v.(string)
		uid, udid, device, deviceid = DecodeNew(token)
		AllData["uid"] = uid
		AllData["udid"] = udid
		AllData["device"] = device
		AllData["deviceid"] = deviceid
		AllData["pre_token"] = ""
	} else {
		AllData["pre_token"] = ""
	}
	raw, _ = json.Marshal(AllData)
	return raw
}

//生成authToken
func EncodeNew(uid int, udid string, device int, deviceid string) (token string, err error) {
	var cipherText []byte
	var Data = "1|" + strconv.FormatInt(time.Now().Unix(), 10) + "|" + "0" + "|" + strconv.Itoa(uid) + "|" + udid + "|" + strconv.Itoa(device) + "|" + deviceid
	var Salt = "xbl2021"
	key := md5.Sum([]byte(Salt))
	if cipherText, err = Encrypt([]byte(Data), key[:], reverse(key[:])); err != nil {
		fmt.Println("aes encrypt error %w", err)
	}
	md5Sum := md5.Sum(cipherText)
	authByte := make([]byte, 0, len(cipherText)+16)
	authByte = append(authByte, md5Sum[:8]...)
	authByte = append(authByte, cipherText...)
	authByte = append(authByte, md5Sum[8:]...)
	return base64.RawURLEncoding.EncodeToString(authByte), err
}

//解密authToken
func DecodeNew(Token string) (uid int, udid string, device int, deviceid string) {
	var err error
	var data []byte
	var Salt = "xbl2021"
	key := md5.Sum([]byte(Salt))
	authByte, _ := base64.RawURLEncoding.DecodeString(Token)
	size := len(authByte)
	plainText := authByte[8 : size-8]
	if data, err = Decrypt(plainText, key[:], reverse(key[:])); err != nil {
		fmt.Println("aes decrypt error %w", err)
	}
	dataString := string(data)
	Arr := strings.Split(dataString, "|")
	if len(Arr) < 7 {
		return 0, "", -1, ""
	}
	uid, _ = strconv.Atoi(Arr[3])
	device, _ = strconv.Atoi(Arr[5])
	return uid, Arr[4], device, Arr[6]
}

//判断四个参数是否为空
func CheckParam(uid int, udid string, device int, deviceid string) bool {
	i := 0
	if uid > 0 {
		i++
	}
	if udid != "" {
		i++
	}
	if device > -1 {
		i++
	}
	if deviceid != "" {
		i++
	}
	if i > 1 {
		return true
	}
	return false
}

func reverse(src []byte) []byte {
	var dest = make([]byte, len(src))
	for i, j := 0, len(src)-1; j >= 0; i, j = i+1, j-1 {
		dest[i] = src[j]
	}
	return dest
}
