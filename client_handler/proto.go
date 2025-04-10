package client_handler

import "github.com/gonet2/agent/misc/packet"

// This file defines the communication structures between client and service.
// Comments must be on their own line!!!!
//
// Basic types: integer float string boolean
// Format as shown below, if you need to define arrays, see array definition examples
//
// Each definition starts with '
// Followed by a comment line describing what this logical structure is for
// Then define the structure name, ending with '=', so you can grep '=' to get all logical names
// After that, each line represents one member definition
//
// Make sure these parts are up to date before publishing the code
//
// Public structure: used to pass only an id, or a simple numeric structure
type S_auto_id struct {
        F_id int32
}

func (p S_auto_id) Pack(w *packet.Packet) {
        w.WriteS32(p.F_id)

}

// General response payload, 0 means success
type S_error_info struct {
        F_code int32
        F_msg  string
}

func (p S_error_info) Pack(w *packet.Packet) {
        w.WriteS32(p.F_code)
        w.WriteString(p.F_msg)

}

// User login package: 1 means login with uuid, 2 means login with client certificate
type S_user_login_info struct {
        F_login_way          int32
        F_open_udid          string
        F_client_certificate string
        F_client_version     int32
        F_user_lang          string
        F_app_id             string
        F_os_version         string
        F_device_name        string
        F_device_id          string
        F_device_id_type     int32
        F_login_ip           string
}

func (p S_user_login_info) Pack(w *packet.Packet) {
        w.WriteS32(p.F_login_way)
        w.WriteString(p.F_open_udid)
        w.WriteString(p.F_client_certificate)
        w.WriteS32(p.F_client_version)
        w.WriteString(p.F_user_lang)
        w.WriteString(p.F_app_id)
        w.WriteString(p.F_os_version)
        w.WriteString(p.F_device_name)
        w.WriteString(p.F_device_id)
        w.WriteS32(p.F_device_id_type)
        w.WriteString(p.F_login_ip)

}

// Communication encryption seed
type S_seed_info struct {
        F_client_send_seed    int32
        F_client_receive_seed int32
}

func (p S_seed_info) Pack(w *packet.Packet) {
        w.WriteS32(p.F_client_send_seed)
        w.WriteS32(p.F_client_receive_seed)

}

// User information package
type S_user_snapshot struct {
        F_uid int32
}

func (p S_user_snapshot) Pack(w *packet.Packet) {
        w.WriteS32(p.F_uid)

}
func PKT_auto_id(reader *packet.Packet) (tbl S_auto_id, err error) {
        tbl.F_id, err = reader.ReadS32()
        checkErr(err)

        return
}

func PKT_error_info(reader *packet.Packet) (tbl S_error_info, err error) {
        tbl.F_code, err = reader.ReadS32()
        checkErr(err)

        tbl.F_msg, err = reader.ReadString()
        checkErr(err)

        return
}

func PKT_user_login_info(reader *packet.Packet) (tbl S_user_login_info, err error) {
        tbl.F_login_way, err = reader.ReadS32()
        checkErr(err)

        tbl.F_open_udid, err = reader.ReadString()
        checkErr(err)

        tbl.F_client_certificate, err = reader.ReadString()
        checkErr(err)

        tbl.F_client_version, err = reader.ReadS32()
        checkErr(err)

        tbl.F_user_lang, err = reader.ReadString()
        checkErr(err)

        tbl.F_app_id, err = reader.ReadString()
        checkErr(err)

        tbl.F_os_version, err = reader.ReadString()
        checkErr(err)

        tbl.F_device_name, err = reader.ReadString()
        checkErr(err)

        tbl.F_device_id, err = reader.ReadString()
        checkErr(err)

        tbl.F_device_id_type, err = reader.ReadS32()
        checkErr(err)

        tbl.F_login_ip, err = reader.ReadString()
        checkErr(err)

        return
}

func PKT_seed_info(reader *packet.Packet) (tbl S_seed_info, err error) {
        tbl.F_client_send_seed, err = reader.ReadS32()
        checkErr(err)

        tbl.F_client_receive_seed, err = reader.ReadS32()
        checkErr(err)

        return
}

func PKT_user_snapshot(reader *packet.Packet) (tbl S_user_snapshot, err error) {
        tbl.F_uid, err = reader.ReadS32()
        checkErr(err)

        return
}

func checkErr(err error) {
        if err != nil {
                panic("error occurred in protocol module")
        }
}
