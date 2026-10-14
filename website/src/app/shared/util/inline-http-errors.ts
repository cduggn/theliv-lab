import { HttpContextToken } from '@angular/common/http';

// These views render scoped errors and retry controls alongside their results.
export const INLINE_HTTP_ERRORS = new HttpContextToken<boolean>(() => false);
