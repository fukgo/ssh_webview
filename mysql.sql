use go;

create table user (
    id int primary key auto_increment,
    username varchar(50) not null
);

create table ssh (
    id int primary key auto_increment,
    -- user_id int not null,
    host varchar(50) not null,
    port int not null,
    user varchar(50) not null,
    password varchar(50),
    private_key TEXT
);