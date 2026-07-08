FROM golang:1.26 AS build
WORKDIR /src

COPY ./ /src

#RUN go build -o /bin/fortigate-lb-controller /src/cmd/fortigate-lb-controller.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/fortigate-lb-controller /src/cmd/fortigate-lb-controller.go

FROM scratch
COPY --from=build /bin/fortigate-lb-controller /bin/fortigate-lb-controller
CMD ["/bin/fortigate-lb-controller"]