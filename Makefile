teardown:
	- docker ps -aq | xargs docker rm -f
	- pkill cockroach mysql
	- k3d cluster delete local
	- docker rm cockroach -f
	- rm -rf crdb/