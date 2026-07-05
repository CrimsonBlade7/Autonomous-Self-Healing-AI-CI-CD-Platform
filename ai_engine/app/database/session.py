from collections.abc import AsyncGenerator

from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy.orm import DeclarativeBase

from app.core.config import settings

# create_async_engine builds a connection pool for the given database URL.
# echo=False means SQL statements are not printed to stdout (set True to debug queries).
engine = create_async_engine(settings.database_url, echo=False)

# async_sessionmaker is the factory that produces AsyncSession objects.
# expire_on_commit=False keeps ORM objects usable after a commit without
# requiring an extra round-trip to the database.
AsyncSessionLocal = async_sessionmaker(engine, expire_on_commit=False)


class Base(DeclarativeBase):
    """
    Shared declarative base for all SQLAlchemy ORM models in this service.
    Any model class should inherit from Base to be tracked by the metadata.
    """
    pass


async def get_session() -> AsyncGenerator[AsyncSession, None]:
    """
    FastAPI dependency that yields an AsyncSession for the duration of a request
    and guarantees the session is closed afterward.

    Usage in a route:
        async def my_route(session: AsyncSession = Depends(get_session)):
            ...
    """
    async with AsyncSessionLocal() as session:
        yield session
