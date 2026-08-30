from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """
    Reads configuration from environment variables or an optional .env file.
    All values can be overridden via docker-compose environment: blocks.
    """

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # PostgreSQL — asyncpg driver is required for SQLAlchemy async sessions.
    # Format: postgresql+asyncpg://user:password@host:port/dbname
    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/aiplatform"

    # Redis — used as Celery broker and result backend.
    # Format: redis://host:port/db_index
    redis_url: str = "redis://localhost:6379/0"

    # Anthropic API key for Claude — used by the patch generator (Phase 4).
    anthropic_api_key: str = ""

    # sentence-transformers model used to create vector embeddings.
    # all-MiniLM-L6-v2 produces 384-dimensional vectors.
    embedding_model: str = "all-MiniLM-L6-v2"
    embedding_dimensions: int = 384


settings = Settings()
