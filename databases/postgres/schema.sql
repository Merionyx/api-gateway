--
-- PostgreSQL database dump
--

-- Dumped from database version 14.19 (Debian 14.19-1.pgdg13+1)
-- Dumped by pg_dump version 17.6 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: control_plane; Type: SCHEMA; Schema: -; Owner: postgres
--

CREATE SCHEMA control_plane;


ALTER SCHEMA control_plane OWNER TO postgres;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: postgres
--

-- *not* creating schema, since initdb creates it


ALTER SCHEMA public OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: environments; Type: TABLE; Schema: control_plane; Owner: postgres
--

CREATE TABLE control_plane.environments (
    uuid uuid NOT NULL,
    name character varying(1024) DEFAULT NULL::character varying NOT NULL,
    config jsonb NOT NULL
);


ALTER TABLE control_plane.environments OWNER TO postgres;

--
-- Name: listeners; Type: TABLE; Schema: control_plane; Owner: postgres
--

CREATE TABLE control_plane.listeners (
    uuid uuid NOT NULL,
    name character varying(1024) DEFAULT NULL::character varying NOT NULL,
    config jsonb NOT NULL
);


ALTER TABLE control_plane.listeners OWNER TO postgres;

--
-- Name: listeners_environments; Type: TABLE; Schema: control_plane; Owner: postgres
--

CREATE TABLE control_plane.listeners_environments (
    listener_uuid uuid NOT NULL,
    environment_uuid uuid NOT NULL
);


ALTER TABLE control_plane.listeners_environments OWNER TO postgres;

--
-- Name: tenants; Type: TABLE; Schema: control_plane; Owner: postgres
--

CREATE TABLE control_plane.tenants (
    uuid uuid NOT NULL,
    name character varying(1024) DEFAULT NULL::character varying NOT NULL
);


ALTER TABLE control_plane.tenants OWNER TO postgres;

--
-- Name: tenants_environments; Type: TABLE; Schema: control_plane; Owner: postgres
--

CREATE TABLE control_plane.tenants_environments (
    tenant_uuid uuid NOT NULL,
    environment_uuid uuid NOT NULL
);


ALTER TABLE control_plane.tenants_environments OWNER TO postgres;

--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


ALTER TABLE public.schema_migrations OWNER TO postgres;

--
-- Name: environments environments_pkey; Type: CONSTRAINT; Schema: control_plane; Owner: postgres
--

ALTER TABLE ONLY control_plane.environments
    ADD CONSTRAINT environments_pkey PRIMARY KEY (uuid);


--
-- Name: listeners_environments listeners_environments_pkey; Type: CONSTRAINT; Schema: control_plane; Owner: postgres
--

ALTER TABLE ONLY control_plane.listeners_environments
    ADD CONSTRAINT listeners_environments_pkey PRIMARY KEY (listener_uuid, environment_uuid);


--
-- Name: listeners listeners_pkey; Type: CONSTRAINT; Schema: control_plane; Owner: postgres
--

ALTER TABLE ONLY control_plane.listeners
    ADD CONSTRAINT listeners_pkey PRIMARY KEY (uuid);


--
-- Name: tenants_environments tenants_environments_pkey; Type: CONSTRAINT; Schema: control_plane; Owner: postgres
--

ALTER TABLE ONLY control_plane.tenants_environments
    ADD CONSTRAINT tenants_environments_pkey PRIMARY KEY (tenant_uuid, environment_uuid);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: control_plane; Owner: postgres
--

ALTER TABLE ONLY control_plane.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (uuid);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: SCHEMA public; Type: ACL; Schema: -; Owner: postgres
--

REVOKE USAGE ON SCHEMA public FROM PUBLIC;
GRANT ALL ON SCHEMA public TO PUBLIC;


--
-- PostgreSQL database dump complete
--

