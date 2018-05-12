all:
	hugo serve

build:
	hugo

api:
	doxygen
	node api.js
	rm -rf doxygen/
