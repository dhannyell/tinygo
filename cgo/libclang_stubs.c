
// This file implements some small trampoline functions. The signatures
// are slightly different from the ones defined in libclang.go, but they
// should be ABI compatible.

#include <clang-c/Index.h> // If this fails, libclang headers aren't available. Please take a look here: https://tinygo.org/docs/guides/build/

CXCursor tinygo_clang_getTranslationUnitCursor(CXTranslationUnit tu) {
	return clang_getTranslationUnitCursor(tu);
}

unsigned tinygo_clang_visitChildren(CXCursor parent, CXCursorVisitor visitor, CXClientData client_data) {
	return clang_visitChildren(parent, visitor, client_data);
}

CXString tinygo_clang_getCursorSpelling(CXCursor c) {
	return clang_getCursorSpelling(c);
}

CXString tinygo_clang_getCursorPrettyPrinted(CXCursor c, CXPrintingPolicy policy) {
	return clang_getCursorPrettyPrinted(c, policy);
}

CXPrintingPolicy tinygo_clang_getCursorPrintingPolicy(CXCursor c) {
	return clang_getCursorPrintingPolicy(c);
}

enum CXCursorKind tinygo_clang_getCursorKind(CXCursor c) {
	return clang_getCursorKind(c);
}

CXType tinygo_clang_getCursorType(CXCursor c) {
	return clang_getCursorType(c);
}

CXCursor tinygo_clang_getTypeDeclaration(CXType t) {
	return clang_getTypeDeclaration(t);
}

CXType tinygo_clang_getTypedefDeclUnderlyingType(CXCursor c) {
	return clang_getTypedefDeclUnderlyingType(c);
}

CXType tinygo_clang_getCursorResultType(CXCursor c) {
	return clang_getCursorResultType(c);
}

int tinygo_clang_Cursor_getNumArguments(CXCursor c) {
	return clang_Cursor_getNumArguments(c);
}

CXCursor tinygo_clang_Cursor_getArgument(CXCursor c, unsigned i) {
	return clang_Cursor_getArgument(c, i);
}

enum CX_StorageClass tinygo_clang_Cursor_getStorageClass(CXCursor c) {
	return clang_Cursor_getStorageClass(c);
}

CXSourceLocation tinygo_clang_getCursorLocation(CXCursor c) {
	return clang_getCursorLocation(c);
}

CXSourceRange tinygo_clang_getCursorExtent(CXCursor c) {
	return clang_getCursorExtent(c);
}

CXTranslationUnit tinygo_clang_Cursor_getTranslationUnit(CXCursor c) {
	return clang_Cursor_getTranslationUnit(c);
}

long long tinygo_clang_getEnumConstantDeclValue(CXCursor c) {
	return clang_getEnumConstantDeclValue(c);
}

CXType tinygo_clang_getEnumDeclIntegerType(CXCursor c) {
	return clang_getEnumDeclIntegerType(c);
}

unsigned tinygo_clang_Cursor_isAnonymous(CXCursor c) {
	return clang_Cursor_isAnonymous(c);
}

unsigned tinygo_clang_Cursor_isBitField(CXCursor c) {
	return clang_Cursor_isBitField(c);
}

unsigned tinygo_clang_Cursor_isMacroFunctionLike(CXCursor c) {
	return clang_Cursor_isMacroFunctionLike(c);
}

static enum CXChildVisitResult tinygo_clang_findDeclRef_visitor(CXCursor c, CXCursor parent, CXClientData client_data) {
	if (clang_getCursorKind(c) == CXCursor_DeclRefExpr) {
		*(CXCursor *)client_data = clang_getCursorReferenced(c);
		return CXChildVisit_Break;
	}
	return CXChildVisit_Recurse;
}

// Return the declaration that the first DeclRefExpr below c refers to, or a
// null cursor.
CXCursor tinygo_clang_findReferencedDecl(CXCursor c) {
	CXCursor result = clang_getNullCursor();
	clang_visitChildren(c, tinygo_clang_findDeclRef_visitor, &result);
	return result;
}
