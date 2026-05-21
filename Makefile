.PHONY: docker run

docker: 
	docker build -t atw-dashbaord:latest .

run: docker
	docker run -p 8080:8080 atw-dashbaord:latest