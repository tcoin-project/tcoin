#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "tcoin.h"

const uint64_t TOTAL_SUPPLY = 1'000'000'000'000'000'000ull;

const char *name() { return "ABC Coin"; }

const char *symbol() { return "ABC"; }

uint8_t decimals() { return 9; }

uint64_t totalSupply() { return TOTAL_SUPPLY; }

uint64_t balanceOf(Address *addr) {
  storageMap<Address, uint64_t> balance_(1);
  return balance_[*addr];
}

bool _transfer(const Address &from, const Address &to, uint64_t value) {
  storageMap<Address, uint64_t> balance_(1);
  auto fromBalance = balance_[from];
  if (fromBalance < value)
    return false;
  fromBalance = fromBalance - value;
  auto toBalance = balance_[to];
  toBalance = toBalance + value;
  return true;
}

bool transfer(const Address *to, uint64_t value) {
  return _transfer(msg::caller(), *to, value);
}

bool transferFrom(const Address *from, const Address *to, uint64_t value) {
  storageMap<Address, storageMap<Address, uint64_t>> allowance_(2);
  auto allowance = allowance_[*from][msg::caller()];
  if (allowance < value)
    return false;
  allowance = allowance - value;
  return _transfer(*from, *to, value);
}

bool approve(const Address *spender, uint64_t value) {
  storageMap<Address, storageMap<Address, uint64_t>> allowance_(2);
  auto allowance = allowance_[msg::caller()][*spender];
  allowance = value;
  return true;
}

uint64_t allowance(const Address *owner, const Address *spender) {
  storageMap<Address, storageMap<Address, uint64_t>> allowance_(2);
  return allowance_[*owner][*spender];
}

const void *entrypoint(uint32_t callId, void *callData) {
  export(name);
  export(symbol);
  export(decimals);
  export(totalSupply);
  export(balanceOf);
  export(transfer);
  export(transferFrom);
  export(approve);
  export(allowance);
  return 0;
}

void regularInit(const void *data) {}

void init() {
  storageMap<Address, uint64_t> balance_(1);
  balance_[msg::caller()] = TOTAL_SUPPLY;
}