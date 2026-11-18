"""OCS storage quota manager package."""
from quota.manager import QuotaExceededError, QuotaManager

__all__ = ["QuotaManager", "QuotaExceededError"]
