CREATE TABLE IF NOT EXISTS tracked_stocks (symbol VARCHAR PRIMARY KEY);

CREATE TABLE IF NOT EXISTS stock_prices (
    symbol VARCHAR REFERENCES tracked_stocks(symbol),
    date TIMESTAMP,
    open DOUBLE,
    high DOUBLE,
    low DOUBLE,
    close DOUBLE,
    volume INTEGER,
    PRIMARY KEY (symbol, date)
);
CREATE TABLE IF NOT EXISTS portfolios (
    user_id VARCHAR,
symbol VARCHAR REFERENCES tracked_stocks(symbol),
    shares DOUBLE,
    PRIMARY KEY (user_id, symbol)
);

CREATE TABLE IF NOT EXISTS watchlists (
    user_id VARCHAR,
symbol VARCHAR REFERENCES tracked_stocks(symbol),
    price_target DOUBLE,
    direction BOOLEAN,
    triggered BOOLEAN DEFAULT false,
    PRIMARY KEY (user_id, symbol)
);