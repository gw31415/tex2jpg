package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
	"unsafe"

	texc_pb "github.com/gw31415/texc/proto"
	"google.golang.org/grpc"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			var errmsg string
			switch err := err.(type) {
			case error:
				errmsg = err.Error()
			case string:
				errmsg = err
			default:
				errmsg = fmt.Sprint(err)
			}
			os.Stderr.WriteString(errmsg)
			os.Exit(1)
		}
	}()

	//標準入力から全てバイト列で読み込む
	tex_code, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	//文字列データを抽出して追加
	conn, err := grpc.Dial("texc.amas.dev:3475", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := texc_pb.NewTexcServiceClient(conn)
	stream, err := client.Sync(context.Background())
	if err != nil {
		panic(err)
	}
	tar_data := bytes.NewBuffer([]byte{})
	tar_w := tar.NewWriter(tar_data)
	buf := new(bytes.Buffer)
	size, _ := fmt.Fprintf(buf, `%s`, tex_code)
	tar_w.WriteHeader(&tar.Header{
		Name:    "bot.tex",
		Mode:    0755,
		ModTime: time.Now(),
		Size:    int64(size),
	})
	io.Copy(tar_w, buf)
	tar_w.Close()
	in_pb := new(texc_pb.Input)
	in_pb.Data = make([]byte, 0xff)
	for {
		_, err := tar_data.Read(in_pb.Data)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		stream.Send(in_pb)
	}
	stream.Send(&texc_pb.Input{
		Exec: []string{"latexmk", "bot.tex"},
	})
	stream.Send(&texc_pb.Input{
		Exec: []string{"pdfcrop", "--margins", "10 10 10 10", "bot.pdf", "bot.pdf"},
	})
	stream.Send(&texc_pb.Input{
		Exec: []string{"pdftoppm", "-singlefile", "-jpeg", "-r", "400", "bot.pdf", "bot"},
	})
	stream.Send(&texc_pb.Input{
		Dl: "bot.jpg",
	})
	stream.CloseSend()
	stdout := bytes.NewBufferString("")
	tar_dl_data := bytes.NewBuffer([]byte{})
	for {
		out, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			body_bin := stdout.Bytes()
			body_str := (*(*string)(unsafe.Pointer(&body_bin)))
			if tar_dl_data.Len() == 0 {
				i := strings.Index(body_str, "!")
				if i == -1 {
					panic(body_str)
				}
				panic(body_str[i:])
			}
			panic(err)
		}
		if out.Stdout != nil {
			stdout.Write(out.Stdout)
		}
		if out.Data != nil {
			tar_dl_data.Write(out.Data)
		}
	}
	body_bin := stdout.Bytes()
	body_str := (*(*string)(unsafe.Pointer(&body_bin)))
	if tar_dl_data.Len() == 0 {
		i := strings.Index(body_str, "!")
		if i == -1 {
			panic(body_str)
		}
		panic(body_str[i:])
	}
	jpeg_data := bytes.NewBuffer([]byte{})
	tar_reader := tar.NewReader(tar_dl_data)
	for {
		_, err := tar_reader.Next()
		/*
			if err == io.EOF {
				break
			}
		*/
		if err != nil {
			break
			//ここらへんバグ
			//panic(err)
		}
		io.Copy(jpeg_data, tar_dl_data)
	}
	os.Stdout.Write(jpeg_data.Bytes())
}
