#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "tcoin.h"

void deposit() {
  storageMap<Address, uint64_t> balance_(1);
  auto balance = balance_[msg::caller()];
  balance = balance + msg::value();
}

void withdraw(uint64_t value) {
  storageMap<Address, uint64_t> balance_(1);
  auto balance = balance_[msg::caller()];
  require(balance >= value, "balance too low");
  balance = balance - value;
  msg::caller().transfer(value, "");
}

uint64_t total() { return self().balance(); }

uint64_t balanceOf(const Address *addr) {
  storageMap<Address, uint64_t> balance_(1);
  return balance_[*addr];
}

uint64_t test(uint64_t l, uint64_t r) {
  uint64_t value = msg::value();
  return value >= l && value <= r ? value : 0;
}

uint64_t testLotsOfArgs(uint64_t a, uint64_t b, uint64_t c, uint64_t d,
                        uint64_t e, uint64_t f, uint64_t g) {
  return a ^ b ^ c ^ d ^ e ^ f ^ g;
}

const void *entrypoint(uint32_t callId, void *callData) {
  export(deposit);
  export(withdraw);
  export(total);
  export(balanceOf);
  export(test);
  export(testLotsOfArgs);
  return 0;
}

void init() {}