#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "tcoin.h"

const char *name() { return "Wrapped TCoin"; }

const char *symbol() { return "WTCoin"; }

uint8_t decimals() { return 9; }

uint64_t totalSupply() {
  Address key(3), val;
  storage::load(key, val.s);
  char *t = val.s;
  uint64_t res;
  deserialize(t, res);
  return res;
}

void adjustTotalSupply(uint64_t diff) {
  Address key(3), val;
  storage::load(key, val.s);
  char *t = val.s;
  uint64_t res;
  deserialize(t, res);
  t = val.s;
  serialize(t, res + diff);
  storage::store(key, val);
}

uint64_t balanceOf(Address *addr) {
  storageMap<Address, uint64_t> balance_(1);
  return balance_[*addr];
}

bool _transfer(const Address &from, const Address &to, uint64_t value) {
  storageMap<Address, uint64_t> balance_(1);
  auto fromBalance = balance_[from];
  uint64_t fromBalance_v = fromBalance;
  if (fromBalance_v < value)
    return false;
  fromBalance = fromBalance_v - value;
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
  uint64_t allowance_v = allowance;
  if (allowance_v < value)
    return false;
  allowance = allowance_v - value;
  return _transfer(*from, *to, value);
}

bool approve(const Address *spender, uint64_t value) {
  storageMap<Address, storageMap<Address, uint64_t>> allowance_(2);
  auto allowance = allowance_[msg::caller()][*spender];
  uint64_t allowance_v = allowance;
  if (!checkAdd(allowance_v, value))
    return false;
  allowance = allowance_v + value;
  return true;
}

uint64_t allowance(const Address *owner, const Address *spender) {
  storageMap<Address, storageMap<Address, uint64_t>> allowance_(2);
  return allowance_[*owner][*spender];
}

void mint() {
  uint64_t val = msg::value();
  adjustTotalSupply(val);
  _transfer(Address(0), msg::caller(), val);
}

bool burn(uint64_t value) {
  if (_transfer(msg::caller(), Address(0), value)) {
    adjustTotalSupply(-value);
    return true;
  }
  return false;
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
  export(mint);
  export(burn);
  return 0;
}

void init() {
  storageMap<Address, uint64_t> balance_(1);
  balance_[Address(0)] = -1ull;
}