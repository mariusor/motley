package env

import "testing"

func TestType_IsProd(t *testing.T) {
	prod := PROD
	if !prod.IsProd() {
		t.Errorf("%T %s should have been production", prod, prod)
	}
	dev := DEV
	if dev.IsProd() {
		t.Errorf("%T %s should not have been production", dev, dev)
	}
	test := TEST
	if test.IsProd() {
		t.Errorf("%T %s should not have been production", test, test)
	}
	rand := Type("Random")
	if rand.IsProd() {
		t.Errorf("%T %s should not have been production", rand, rand)
	}
}

func TestType_IsTest(t *testing.T) {
	test := TEST
	if !test.IsTest() {
		t.Errorf("%T %s should have been test", test, test)
	}
	prod := PROD
	if prod.IsTest() {
		t.Errorf("%T %s should not have been test", prod, prod)
	}
	dev := DEV
	if dev.IsTest() {
		t.Errorf("%T %s should not have been test", dev, dev)
	}
	rand := Type("Random")
	if rand.IsTest() {
		t.Errorf("%T %s should not have been test", rand, rand)
	}
}

func TestValidTypeOrDev(t *testing.T) {
	prod := PROD
	if prod != ValidTypeOrDev(prod) {
		t.Errorf("%T %s should have been valid, received %s", prod, prod, ValidTypeOrDev(prod))
	}
	test := TEST
	if test != ValidTypeOrDev(test) {
		t.Errorf("%T %s should have been valid, received %s", test, test, ValidTypeOrDev(test))
	}
	dev := DEV
	if dev != ValidTypeOrDev(dev) {
		t.Errorf("%T %s should have been valid, received %s", dev, dev, ValidTypeOrDev(dev))
	}
	rand := "Random"
	if dev != ValidTypeOrDev(Type(rand)) {
		t.Errorf("%T %s should not have been valid, received %s", rand, rand, ValidTypeOrDev(Type(rand)))
	}
}

func TestValidType(t *testing.T) {
	prod := PROD
	if !ValidType(prod) {
		t.Errorf("%T %s should have been valid", prod, prod)
	}
	dev := DEV
	if !ValidType(dev) {
		t.Errorf("%T %s should have been valid", dev, dev)
	}
	test := TEST
	if !ValidType(test) {
		t.Errorf("%T %s should have been valid", test, test)
	}
	rand := "Random"
	if ValidType(Type(rand)) {
		t.Errorf("%T %s should not have been valid", Type(rand), rand)
	}
}
