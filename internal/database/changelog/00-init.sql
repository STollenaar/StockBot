CREATE TABLE IF NOT EXISTS portfolios (
    user_id VARCHAR,
    symbol VARCHAR,
    shares DOUBLE,
    PRIMARY KEY (user_id, symbol)
);

CREATE TABLE IF NOT EXISTS watchlists (
    user_id VARCHAR,
    symbol VARCHAR,
    price_target DOUBLE,
    direction BOOLEAN,
    triggered BOOLEAN DEFAULT false,
    PRIMARY KEY (user_id, symbol)
);