go tool vet -printfuncs=httpErrorf:1,panicif:1,Noticef,Errorf .
go build -o sumatra_website.exe
.\sumatra_website.exe
del sumatra_website.exe
