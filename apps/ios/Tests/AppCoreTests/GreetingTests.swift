import Testing

@testable import AppCore

// A test names its acceptance scenario in its title, e.g. @Test("001/US1-AS2 signs out") (specs-scenarios-tested).
@Test("greets by name")
func greetsByName() {
    #expect(greeting(for: "Ada") == "Hello, Ada")
}

@Test("greets without a name")
func greetsWithoutName() {
    #expect(greeting(for: "") == "Hello")
}
