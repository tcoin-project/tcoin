#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "tcoin.h"

struct Test1 : Contract {
  Test1(const Contract &c) { call = c.call; }
  void deposit();
  void withdraw(uint64_t value);
  uint64_t total();
  uint64_t balanceOf(const Address *addr);
  uint64_t testLotsOfArgs(uint64_t a, uint64_t b, uint64_t c, uint64_t d,
                          uint64_t e, uint64_t f, uint64_t g);
};

impl0(Test1::deposit);
impl1(Test1::withdraw);
impl0(Test1::total);
impl1(Test1::balanceOf);
impl7(Test1::testLotsOfArgs);

uint64_t test() {
  Address caller = msg::caller();
  Test1 test = loadContract(&caller);
  // return test.balanceOf(asSharedPtr(self()));
  return test.testLotsOfArgs(1, 2, 3, 4, 5, 6, 7);
}

const void *entrypoint(uint32_t callId, void *callData) {
  export(test);
  return 0;
}

void init(void *initData) {}