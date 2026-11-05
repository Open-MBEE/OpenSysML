using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class TestCategoryTests
{
    private static readonly string[] Categories = [Category.Unit, Category.Integration, Category.Interop];

    [Fact]
    public void EveryTestDeclaresExactlyOneKnownCategory()
    {
        var tests = typeof(TestCategoryTests).Assembly.GetTypes()
            .SelectMany(type => type.GetMethods(BindingFlags.Public | BindingFlags.Instance | BindingFlags.Static | BindingFlags.DeclaredOnly))
            .Where(method => method.IsDefined(typeof(FactAttribute), inherit: true))
            .ToList();
        Assert.NotEmpty(tests);

        var offenders = new List<string>();
        foreach (var test in tests)
        {
            var categories = CategoriesOf(test).Concat(CategoriesOf(test.DeclaringType!)).ToList();
            if (categories.Count != 1 || !Categories.Contains(categories[0], StringComparer.Ordinal))
            {
                offenders.Add($"{test.DeclaringType!.Name}.{test.Name}: [{string.Join(", ", categories)}]");
            }
        }

        Assert.True(
            offenders.Count == 0,
            $"Each test declares one {Category.Name} trait of {string.Join(" or ", Categories)}, on the method or its class:\n"
                + string.Join("\n", offenders));
    }

    private static IEnumerable<string> CategoriesOf(MemberInfo member) =>
        member.GetCustomAttributes<TraitAttribute>(inherit: true)
            .Where(trait => trait.Name == Category.Name)
            .Select(trait => trait.Value);
}
