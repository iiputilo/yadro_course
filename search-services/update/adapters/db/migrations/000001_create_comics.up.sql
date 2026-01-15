CREATE TABLE IF NOT EXISTS comics (
                                      id        INTEGER PRIMARY KEY,
                                      img_url   TEXT    NOT NULL,
                                      words     TEXT[]  NOT NULL
);