#include "map.h"
#include "safemath.h"
#include "stdlib.h"
#include "tcoin.h"

struct Token : Contract {
  Token(const Contract &c) { call = c.call; }
  const char *name();
  const char *symbol();
  uint8_t decimals();
  uint64_t totalSupply();
  uint64_t balanceOf(const Address *addr);
  bool transfer(const Address *to, uint64_t value);
  bool transferFrom(const Address *from, const Address *to, uint64_t value);
  bool approve(const Address *spender, uint64_t value);
  uint64_t allowance(const Address *owner, const Address *spender);
};

impl0(Token::name);
impl0(Token::symbol);
impl0(Token::decimals);
impl0(Token::totalSupply);
impl1(Token::balanceOf);
impl2(Token::transfer);
impl3(Token::transferFrom);
impl2(Token::approve);
impl2(Token::allowance);

PRIVATE_DATA const Address *tokenAddr;

uint64_t msb(uint64_t x) {
  uint64_t res = 0;
  if (x >> 32)
    x >>= 32, res += 32;
  if (x >> 16)
    x >>= 16, res += 16;
  if (x >> 8)
    x >>= 8, res += 8;
  if (x >> 4)
    x >>= 4, res += 4;
  if (x >> 2)
    x >>= 2, res += 2;
  if (x >> 1)
    x >>= 1, res += 1;
  return res;
}

struct bigint {
  union {
    uint64_t s64[4];
    uint32_t s[8];
  };
  bigint() { s64[0] = s64[1] = s64[2] = s64[3] = 0; }
  bigint(uint64_t x) {
    s64[0] = x;
    s64[1] = s64[2] = s64[3] = 0;
  }
  bigint(uint64_t x, uint64_t shift) {
    s64[0] = s64[1] = s64[2] = s64[3] = 0;
    if (shift % 64 == 0) {
      s64[shift / 64] = x;
    } else {
      uint64_t t = shift / 64;
      s64[t] = x << (shift & 63);
      if (t < 3)
        s64[t + 1] = x >> (64 - (shift & 63));
    }
  }
  bigint(const bigint &x) {
    for (int i = 0; i < 4; i++)
      s64[i] = x.s64[i];
  }
  bigint &operator=(const bigint &x) {
    for (int i = 0; i < 4; i++)
      s64[i] = x.s64[i];
    return *this;
  }
  bigint &operator+=(const bigint &o) {
    uint64_t last = 0;
    for (int i = 0; i < 8; i++) {
      uint64_t tmp = last + (uint64_t)s[i] + (uint64_t)o.s[i];
      s[i] = tmp;
      last = tmp >> 32;
    }
    return *this;
  }
  bigint &operator-=(const bigint &o) { return *this += -o; }
  bigint &operator*=(const bigint &o) {
    uint64_t sum[8];
    for (int i = 0; i < 8; i++)
      sum[i] = 0;
    for (int i = 0; i < 8; i++)
      for (int j = 0; j < 8 - i; j++) {
        uint64_t tmp = (uint64_t)s[i] * (uint64_t)o.s[j];
        sum[i + j] += uint32_t(tmp);
        if (i + j < 7)
          sum[i + j + 1] += tmp >> 32;
      }
    uint64_t last = 0;
    for (int i = 0; i < 8; i++) {
      uint64_t tmp = sum[i] + last;
      s[i] = tmp;
      last = tmp >> 32;
    }
    return *this;
  }
  const bigint operator-() const {
    bigint t;
    for (int i = 0; i < 4; i++)
      t.s64[i] = ~s64[i];
    return t += bigint(1);
  }
  const bigint operator+(const bigint &o) const {
    bigint t = *this;
    return t += o;
  }
  const bigint operator-(const bigint &o) const {
    bigint t = *this;
    return t += o;
  }
  const bigint operator*(const bigint &o) const {
    bigint t = *this;
    return t *= o;
  }
  bool operator<(const bigint &o) const {
    for (int i = 3; i >= 0; i--) {
      if (s64[i] != o.s64[i])
        return s64[i] < o.s64[i];
    }
    return false;
  }
  bool operator>(const bigint &o) const { return o < *this; }
  bool operator<=(const bigint &o) const { return !(*this > o); }
  bool operator>=(const bigint &o) const { return !(*this < o); }

  struct msbits {
    uint64_t bits, shift;
  };
  msbits getMsbits() const {
    for (int i = 3; i; i--) {
      if (s64[i]) {
        uint64_t a = s64[i], x = msb(a);
        if (x == 63)
          return msbits{a, uint64_t(i << 6)};
        uint64_t b = a << (63 - x) | s64[i - 1] >> (x + 1);
        return msbits{b, uint64_t((i - 1) << 6 | (x + 1))};
      }
    }
    return msbits{s64[0], 0};
  }

  static bigint divide(bigint a, const bigint &b) {
    bigint res, cur;
    while (a >= b) {
      msbits ax = a.getMsbits(), bx = b.getMsbits();
      uint64_t shift = ax.shift - bx.shift;
      uint64_t ushift = shift >> 1;
      if (ushift > 32)
        ushift = 32;
      uint64_t bu = bx.bits >> ushift;
      if (ushift && bu) {
        uint64_t tmp = ax.bits / (bu + 1);
        if (tmp) {
          cur = bigint(tmp, shift - ushift);
        } else {
          cur = 1;
        }
      } else {
        uint64_t tmp = ax.bits / (bx.bits + 1);
        if (tmp) {
          cur = bigint(tmp, shift);
        } else {
          cur = 1;
        }
      }
      res += cur;
      a -= cur * b;
    }
    return res;
  }

  const bigint operator/(const bigint &o) const {
    return bigint::divide(*this, o);
  }

  operator uint64_t() const { return s64[0]; }
};

uint64_t addLiquidity(uint64_t minLiquidity, uint64_t maxTokens) {
  storageMap<Address, uint64_t> balance_(1);
  storageVar<uint64_t> totalSupply_(3);
  assert(maxTokens > 0 && msg::value() > 0);
  uint64_t totalLiquidity = totalSupply_;
  Token token(loadContract(tokenAddr));
  if (totalLiquidity > 0) {
    uint64_t tcoinReserve = self().balance() - msg::value();
    uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
    uint64_t tokenAmount =
        uint64_t(bigint(msg::value()) * bigint(tokenReserve) /
                 bigint(tcoinReserve)) +
        1;
    uint64_t liquidityMinted = uint64_t(
        bigint(msg::value()) * bigint(totalLiquidity) / bigint(tcoinReserve));
    assert(maxTokens >= tokenAmount && liquidityMinted >= minLiquidity);
    auto balance = balance_[msg::caller()];
    balance = balance + liquidityMinted;
    totalSupply_ = totalLiquidity + liquidityMinted;
    assert(token.transferFrom(asSharedPtr(msg::caller()), asSharedPtr(self()),
                              tokenAmount));
    return liquidityMinted;
  } else {
    assert(msg::value() >= 1000000000);
    uint64_t tokenAmount = maxTokens;
    uint64_t initialLiquidity = self().balance();
    totalSupply_ = initialLiquidity;
    balance_[msg::caller()] = initialLiquidity;
    assert(token.transferFrom(asSharedPtr(msg::caller()), asSharedPtr(self()),
                              tokenAmount));
    return initialLiquidity;
  }
}

struct removeLiquidityResult {
  uint64_t tcoinAmount, tokenAmount;
};

const removeLiquidityResult *removeLiquidity(uint64_t amount, uint64_t minTcoin,
                                             uint64_t minTokens) {
  storageMap<Address, uint64_t> balance_(1);
  storageVar<uint64_t> totalSupply_(3);
  assert(minTcoin > 0 && minTokens > 0);
  uint64_t totalLiquidity = totalSupply_;
  Token token(loadContract(tokenAddr));
  assert(totalLiquidity > 0);
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  uint64_t tcoinAmount =
      bigint(amount) * bigint(self().balance()) / bigint(totalLiquidity);
  uint64_t tokenAmount =
      bigint(amount) * bigint(tokenReserve) / bigint(totalLiquidity);
  assert(tcoinAmount >= minTcoin && tokenAmount >= minTokens);
  auto balance = balance_[msg::caller()];
  balance = balance - amount;
  totalSupply_ = totalLiquidity - amount;
  msg::caller().transfer(amount, "remove liquidity");
  assert(token.transfer(asSharedPtr(msg::caller()), tokenAmount));
  return asSharedPtr(removeLiquidityResult{tcoinAmount, tokenAmount});
}

uint64_t getInputPrice(uint64_t inputAmount, uint64_t inputReserve,
                       uint64_t outputReserve) {
  assert(inputReserve > 0 && outputReserve > 0);
  bigint inputAmountWithFee = bigint(inputAmount) * bigint(997);
  bigint numerator = inputAmountWithFee * bigint(outputReserve);
  bigint denominator = bigint(inputReserve) * bigint(1000) + inputAmountWithFee;
  return numerator / denominator;
}

uint64_t getOutputPrice(uint64_t outputAmount, uint64_t inputReserve,
                        uint64_t outputReserve) {
  assert(inputReserve > 0 && outputReserve > 0);
  bigint numerator = bigint(inputReserve) * bigint(outputAmount) * bigint(1000);
  bigint denominator = bigint(outputReserve - outputAmount) * bigint(997);
  return uint64_t(numerator / denominator) + 1;
}

uint64_t tcoinToTokenInput(uint64_t tcoinSold, uint64_t minTokens,
                           const Address &buyer) {
  assert(tcoinSold > 0 && minTokens > 0);
  Token token(loadContract(tokenAddr));
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  uint64_t tokensBought =
      getInputPrice(tcoinSold, self().balance() - tcoinSold, tokenReserve);
  assert(tokensBought >= minTokens);
  assert(token.transfer(asSharedPtr(buyer), tokensBought));
  return tokensBought;
}

uint64_t tcoinToTokenSwapInput(uint64_t minTokens) {
  return tcoinToTokenInput(msg::value(), minTokens, msg::caller());
}

uint64_t tcoinToTokenTransferInput(uint64_t minTokens,
                                   const Address *recipient) {
  return tcoinToTokenInput(msg::value(), minTokens, *recipient);
}

uint64_t tcoinToTokenOutput(uint64_t tokensBought, uint64_t maxTcoin,
                            const Address &buyer, const Address &recipient) {
  assert(tokensBought > 0 && maxTcoin > 0);
  Token token(loadContract(tokenAddr));
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  uint64_t tcoinSold =
      getOutputPrice(tokensBought, self().balance() - maxTcoin, tokenReserve);
  uint64_t tcoinRefund = maxTcoin - tcoinSold;
  if (tcoinRefund > 0) {
    buyer.transfer(tcoinRefund, "refund");
  }
  assert(token.transfer(asSharedPtr(recipient), tokensBought));
  return tcoinSold;
}

uint64_t tcoinToTokenSwapOutput(uint64_t tokensBought) {
  return tcoinToTokenOutput(tokensBought, msg::value(), msg::caller(),
                            msg::caller());
}

uint64_t tcoinToTokenTransferOutput(uint64_t tokensBought,
                                    const Address *recipient) {
  return tcoinToTokenOutput(tokensBought, msg::value(), msg::caller(),
                            *recipient);
}

uint64_t tokenToTcoinInput(uint64_t tokensSold, uint64_t minTcoin,
                           const Address &buyer, const Address &recipient) {
  assert(tokensSold > 0 and minTcoin > 0);
  Token token(loadContract(tokenAddr));
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  uint64_t tcoinBought =
      getInputPrice(tokensSold, tokenReserve, self().balance());
  assert(tcoinBought >= minTcoin);
  recipient.transfer(tcoinBought, "sell tokens");
  assert(
      token.transferFrom(asSharedPtr(buyer), asSharedPtr(self()), tokensSold));
  return tcoinBought;
}

uint64_t tokenToTcoinSwapInput(uint64_t tokensSold, uint64_t minTcoin) {
  return tokenToTcoinInput(tokensSold, minTcoin, msg::caller(), msg::caller());
}

uint64_t tokenToTcoinTransferInput(uint64_t tokensSold, uint64_t minTcoin,
                                   const Address *recipient) {
  return tokenToTcoinInput(tokensSold, minTcoin, msg::caller(), *recipient);
}

uint64_t tokenToTcoinOutput(uint64_t tcoinBought, uint64_t maxTokens,
                            const Address &buyer, const Address &recipient) {
  assert(tcoinBought > 0);
  Token token(loadContract(tokenAddr));
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  uint64_t tokensSold =
      getOutputPrice(tcoinBought, tokenReserve, self().balance());
  assert(maxTokens > tokensSold);
  recipient.transfer(tcoinBought, "sell tokens");
  assert(
      token.transferFrom(asSharedPtr(buyer), asSharedPtr(self()), tokensSold));
  return tokensSold;
}

uint64_t tokenToTcoinSwapOutput(uint64_t tcoinBought, uint64_t maxTokens) {
  return tokenToTcoinOutput(tcoinBought, maxTokens, msg::caller(),
                            msg::caller());
}

uint64_t tokenToTcoinTransferOutput(uint64_t tcoinBought, uint64_t maxTokens,
                                    const Address *recipient) {
  return tokenToTcoinOutput(tcoinBought, maxTokens, msg::caller(), *recipient);
}

uint64_t getTcoinToTokenInputPrice(uint64_t tcoinSold) {
  assert(tcoinSold > 0);
  Token token(loadContract(tokenAddr));
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  return getInputPrice(tcoinSold, self().balance(), tokenReserve);
}

uint64_t getTcoinToTokenOutputPrice(uint64_t tokensBought) {
  assert(tokensBought > 0);
  Token token(loadContract(tokenAddr));
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  return getOutputPrice(tokensBought, self().balance(), tokenReserve);
}

uint64_t getTokenToTcoinInputPrice(uint64_t tokensSold) {
  assert(tokensSold > 0);
  Token token(loadContract(tokenAddr));
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  return getInputPrice(tokensSold, tokenReserve, self().balance());
}

uint64_t getTokenToTcoinOutputPrice(uint64_t tcoinBought) {
  assert(tcoinBought > 0);
  Token token(loadContract(tokenAddr));
  uint64_t tokenReserve = token.balanceOf(asSharedPtr(self()));
  return getOutputPrice(tcoinBought, tokenReserve, self().balance());
}

const char *name() { return "Swap Liquidity"; }

const char *symbol() { return "SWAP"; }

uint8_t decimals() { return 9; }

uint64_t totalSupply() {
  storageVar<uint64_t> totalSupply_(3);
  return totalSupply_;
}

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
  if (!checkAdd(allowance, value))
    return false;
  allowance = allowance + value;
  return true;
}

uint64_t allowance(const Address *owner, const Address *spender) {
  storageMap<Address, storageMap<Address, uint64_t>> allowance_(2);
  return allowance_[*owner][*spender];
}

const void *entrypoint(uint32_t callId, void *callData) {
  export(addLiquidity);
  export(removeLiquidity);
  export(tcoinToTokenSwapInput);
  export(tcoinToTokenTransferInput);
  export(tcoinToTokenSwapOutput);
  export(tcoinToTokenTransferOutput);
  export(tokenToTcoinSwapInput);
  export(tokenToTcoinTransferInput);
  export(tokenToTcoinSwapOutput);
  export(tokenToTcoinTransferOutput);
  export(getTcoinToTokenInputPrice);
  export(getTcoinToTokenOutputPrice);
  export(getTokenToTcoinInputPrice);
  export(getTokenToTcoinOutputPrice);
  export(name);
  export(symbol);
  export(decimals);
  export(totalSupply);
  export(balanceOf);
  export(transfer);
  export(transferFrom);
  export(approve);
  export(allowance);
  tcoinToTokenInput(msg::value(), 1, msg::caller());
  return 0;
}

void regularInit(const void *data) {
  tokenAddr = reinterpret_cast<const Address *>(data);
}

void init() {}